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
	tradingv1 "github.com/truthmarket/truth-market/proto/gen/go/trading/v1"
	"github.com/truthmarket/truth-market/services/trading-svc/internal/config"
	tradinggrpc "github.com/truthmarket/truth-market/services/trading-svc/internal/grpc"
	"github.com/truthmarket/truth-market/services/trading-svc/internal/matching"
	"github.com/truthmarket/truth-market/services/trading-svc/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const serviceName = "trading-svc"

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

	// ---------- Repositories ----------
	orderRepo := postgres.NewOrderRepo(pool)
	tradeRepo := postgres.NewTradeRepo(pool)
	positionRepo := postgres.NewPositionRepo(pool)
	userRepo := postgres.NewUserRepo(pool)
	outcomeRepo := postgres.NewOutcomeRepo(pool)
	txManager := postgres.NewTxManager(pool)

	// ---------- Event Bus (Redis) ----------
	eventBus := infraredis.NewEventBus(rdb)
	defer func() {
		if err := eventBus.Close(); err != nil {
			log.Error("event bus close error", "error", err)
		}
	}()

	// ---------- Matching Engine ----------
	engine := matching.NewRegistry()

	// ---------- Services ----------
	orderSvc := service.NewOrderService(orderRepo, userRepo, positionRepo, tradeRepo, txManager, engine)
	mintSvc := service.NewMintService(userRepo, outcomeRepo, positionRepo, tradeRepo, txManager)

	// ---------- gRPC Server ----------
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelmw.GRPCServerHandler()),
	)

	// Register gRPC health check.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus(serviceName, healthpb.HealthCheckResponse_SERVING)

	// Register trading service RPCs.
	tradingSrv := tradinggrpc.NewTradingServer(
		&orderServiceAdapter{svc: orderSvc},
		mintSvc,
		tradinggrpc.WithOrderbookProvider(engine),
	)
	tradingv1.RegisterTradingServiceServer(grpcServer, tradingSrv)

	_ = eventBus // will be used for publishing trade/order events

	// ---------- Listen ----------
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Error("failed to listen", "port", cfg.Port, "error", err)
		os.Exit(1)
	}

	go func() {
		log.Info("trading-svc listening", "port", cfg.Port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server error", "error", err)
			cancel()
		}
	}()

	// ---------- Graceful shutdown ----------
	<-ctx.Done()
	log.Info("shutting down trading-svc")
	grpcServer.GracefulStop()
	log.Info("trading-svc stopped")
}
