package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/truthmarket/truth-market/infra/postgres"
	infraredis "github.com/truthmarket/truth-market/infra/redis"
	"github.com/truthmarket/truth-market/pkg/logger"
	"github.com/truthmarket/truth-market/pkg/otel"
	otelmw "github.com/truthmarket/truth-market/pkg/otel/middleware"
	rankingv1 "github.com/truthmarket/truth-market/proto/gen/go/ranking/v1"
	"github.com/truthmarket/truth-market/services/ranking-svc/internal/config"
	rankinggrpc "github.com/truthmarket/truth-market/services/ranking-svc/internal/grpc"
	"github.com/truthmarket/truth-market/services/ranking-svc/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const serviceName = "ranking-svc"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ---------- Config ----------
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// ---------- Logger ----------
	log := logger.New(
		logger.WithServiceName(serviceName),
	)

	// ---------- OpenTelemetry ----------
	otelShutdown, err := otel.InitProvider(ctx, otel.Config{
		ServiceName: serviceName,
		Endpoint:    cfg.OTelEndpoint,
		Insecure:    true,
	})
	if err != nil {
		log.Error("failed to init otel provider", "error", err)
	}
	defer func() {
		if err := otelShutdown(context.Background()); err != nil {
			log.Error("otel shutdown error", "error", err)
		}
	}()

	// ---------- Postgres ----------
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer postgres.Close(pool)

	// ---------- Redis ----------
	rdb := infraredis.NewClient(cfg.RedisAddr, "", 0)
	defer func() {
		if err := infraredis.Close(rdb); err != nil {
			log.Error("redis close error", "error", err)
		}
	}()

	// ---------- Repos ----------
	rankingRepo := postgres.NewRankingRepo(pool)
	userRepo := postgres.NewUserRepo(pool)
	positionRepo := postgres.NewPositionRepo(pool)
	rankingCache := infraredis.NewRankingCache(rdb)

	// ---------- Services ----------
	rankingSvc := service.NewRankingService(rankingRepo, userRepo)
	portfolioSvc := service.NewPortfolioService(positionRepo, userRepo)

	// Suppress unused-variable lint for rankingCache until it is integrated
	// into a caching layer; the constructor call above validates connectivity.
	_ = rankingCache

	// Refresh the materialized view on startup so rankings reflect
	// any changes that occurred while the service was down.
	if err := rankingRepo.RefreshMaterializedView(ctx); err != nil {
		log.Warn("failed to refresh rankings on startup", "error", err)
	} else {
		log.Info("rankings materialized view refreshed on startup")
	}

	// ---------- gRPC Server ----------
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelmw.GRPCServerHandler()),
	)

	// Register gRPC health check.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus(serviceName, healthpb.HealthCheckResponse_SERVING)

	// Register ranking service RPCs.
	rankingServer := rankinggrpc.NewRankingServer(rankingSvc, &portfolioAdapter{svc: portfolioSvc})
	rankingv1.RegisterRankingServiceServer(grpcServer, rankingServer)

	// ---------- Listen ----------
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Error("failed to listen", "port", cfg.Port, "error", err)
		os.Exit(1)
	}

	go func() {
		log.Info("ranking-svc listening", "port", cfg.Port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server error", "error", err)
			cancel()
		}
	}()

	// ---------- Graceful shutdown ----------
	<-ctx.Done()
	log.Info("shutting down ranking-svc")
	grpcServer.GracefulStop()
	log.Info("ranking-svc stopped")
}

// ---------------------------------------------------------------------------
// portfolioAdapter bridges service.PortfolioService (which returns
// *service.Portfolio) to the rankinggrpc.PortfolioServicer interface (which
// expects *rankinggrpc.Portfolio). The two Portfolio types are structurally
// identical but live in different packages.
// ---------------------------------------------------------------------------

type portfolioAdapter struct {
	svc *service.PortfolioService
}

func (a *portfolioAdapter) GetPortfolio(ctx context.Context, userID string) (*rankinggrpc.Portfolio, error) {
	p, err := a.svc.GetPortfolio(ctx, userID)
	if err != nil {
		return nil, err
	}

	positions := make([]rankinggrpc.PortfolioPosition, len(p.Positions))
	for i, pos := range p.Positions {
		positions[i] = rankinggrpc.PortfolioPosition{
			MarketID:  pos.MarketID,
			OutcomeID: pos.OutcomeID,
			Quantity:  pos.Quantity,
			AvgPrice:  pos.AvgPrice,
			Value:     pos.Value,
		}
	}

	return &rankinggrpc.Portfolio{
		TotalValue:    p.TotalValue,
		UnrealizedPnL: p.UnrealizedPnL,
		Positions:     positions,
	}, nil
}
