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
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/logger"
	"github.com/truthmarket/truth-market/pkg/otel"
	otelmw "github.com/truthmarket/truth-market/pkg/otel/middleware"
	"github.com/truthmarket/truth-market/pkg/repository"
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
	tradingv1 "github.com/truthmarket/truth-market/proto/gen/go/trading/v1"
	"github.com/truthmarket/truth-market/services/market-svc/internal/config"
	marketgrpc "github.com/truthmarket/truth-market/services/market-svc/internal/grpc"
	"github.com/truthmarket/truth-market/services/market-svc/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const serviceName = "market-svc"

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

	// ---------- Trading service client (for settlement) ----------
	tradingConn, err := grpc.NewClient(cfg.TradingSvcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelmw.GRPCClientHandler()),
	)
	if err != nil {
		log.Error("failed to connect to trading service", "error", err)
		os.Exit(1)
	}
	defer tradingConn.Close()

	// ---------- Repositories ----------
	marketRepo := postgres.NewMarketRepo(pool)
	outcomeRepo := postgres.NewOutcomeRepo(pool)
	positionRepo := postgres.NewPositionRepo(pool)
	userRepo := postgres.NewUserRepo(pool)
	txManager := postgres.NewTxManager(pool)

	// ---------- Event Bus (Redis Pub/Sub) ----------
	eventBus := infraredis.NewEventBus(rdb)
	defer func() {
		if err := eventBus.Close(); err != nil {
			log.Error("event bus close error", "error", err)
		}
	}()

	// ---------- Trading service client (order canceller) ----------
	tradingClient := tradingv1.NewTradingServiceClient(tradingConn)
	orderCanceller := &tradingOrderCanceller{client: tradingClient}

	// ---------- Domain Services ----------
	marketService := service.NewMarketService(marketRepo, outcomeRepo, txManager)
	_ = service.NewSettlementService(
		marketRepo, outcomeRepo, positionRepo, userRepo,
		orderCanceller, txManager, eventBus,
	)

	// ---------- gRPC Server ----------
	marketServer := marketgrpc.NewMarketServer(&marketServiceAdapter{svc: marketService})

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelmw.GRPCServerHandler()),
	)

	// Register gRPC health check.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus(serviceName, healthpb.HealthCheckResponse_SERVING)

	// Register market service RPCs.
	marketv1.RegisterMarketServiceServer(grpcServer, marketServer)

	// ---------- Listen ----------
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Error("failed to listen", "port", cfg.Port, "error", err)
		os.Exit(1)
	}

	go func() {
		log.Info("market-svc listening", "port", cfg.Port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server error", "error", err)
			cancel()
		}
	}()

	// ---------- Graceful shutdown ----------
	<-ctx.Done()
	log.Info("shutting down market-svc")
	grpcServer.GracefulStop()
	log.Info("market-svc stopped")
}

// tradingOrderCanceller adapts the trading gRPC client to the
// service.OrderCanceller interface required by SettlementService.
type tradingOrderCanceller struct {
	client tradingv1.TradingServiceClient
}

func (c *tradingOrderCanceller) CancelAllOrdersByMarket(ctx context.Context, marketID string) (int64, error) {
	resp, err := c.client.CancelAllOrdersByMarket(ctx, &tradingv1.CancelAllOrdersByMarketRequest{
		MarketId: marketID,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetCancelledCount(), nil
}

// marketServiceAdapter bridges service.MarketService to the
// grpc.MarketServicer interface by converting the CreateMarketRequest type.
type marketServiceAdapter struct {
	svc *service.MarketService
}

func (a *marketServiceAdapter) CreateMarket(ctx context.Context, req marketgrpc.CreateMarketRequest) (*domain.Market, error) {
	return a.svc.CreateMarket(ctx, service.CreateMarketRequest{
		Title:         req.Title,
		Description:   req.Description,
		MarketType:    req.MarketType,
		Category:      req.Category,
		OutcomeLabels: req.OutcomeLabels,
		EndTime:       req.EndTime,
		CreatedBy:     req.CreatedBy,
	})
}

func (a *marketServiceAdapter) GetMarket(ctx context.Context, id string) (*domain.Market, []*domain.Outcome, error) {
	return a.svc.GetMarket(ctx, id)
}

func (a *marketServiceAdapter) ListMarkets(ctx context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error) {
	return a.svc.ListMarkets(ctx, filter)
}

func (a *marketServiceAdapter) UpdateMarketStatus(ctx context.Context, id string, status domain.MarketStatus) error {
	return a.svc.UpdateMarketStatus(ctx, id, status)
}

func (a *marketServiceAdapter) ResolveMarket(ctx context.Context, marketID, winningOutcomeID string) error {
	return a.svc.ResolveMarket(ctx, marketID, winningOutcomeID)
}
