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
	"github.com/truthmarket/truth-market/services/market-svc/internal/config"
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

	// pool, rdb, tradingConn will be used once repos and services are wired.
	_ = pool
	_ = rdb
	_ = tradingConn

	// ---------- gRPC Server ----------
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelmw.GRPCServerHandler()),
	)

	// Register gRPC health check.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus(serviceName, healthpb.HealthCheckResponse_SERVING)

	// TODO: Register market service RPCs here.

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
