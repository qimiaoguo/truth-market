package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"
	infraredis "github.com/truthmarket/truth-market/infra/redis"
	"github.com/truthmarket/truth-market/pkg/logger"
	"github.com/truthmarket/truth-market/pkg/otel"
	otelmw "github.com/truthmarket/truth-market/pkg/otel/middleware"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
	rankingv1 "github.com/truthmarket/truth-market/proto/gen/go/ranking/v1"
	tradingv1 "github.com/truthmarket/truth-market/proto/gen/go/trading/v1"
	"github.com/truthmarket/truth-market/services/gateway/internal"
	"github.com/truthmarket/truth-market/services/gateway/internal/config"
	"github.com/truthmarket/truth-market/services/gateway/internal/handler"
	"github.com/truthmarket/truth-market/services/gateway/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const serviceName = "gateway"

func main() {
	// Root context cancelled on SIGINT / SIGTERM.
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

	// ---------- gRPC client connections ----------
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelmw.GRPCClientHandler()),
	}

	authConn, err := grpc.NewClient(cfg.AuthSvcAddr, dialOpts...)
	if err != nil {
		log.Error("failed to connect to auth service", "error", err)
		os.Exit(1)
	}
	defer authConn.Close()

	marketConn, err := grpc.NewClient(cfg.MarketSvcAddr, dialOpts...)
	if err != nil {
		log.Error("failed to connect to market service", "error", err)
		os.Exit(1)
	}
	defer marketConn.Close()

	tradingConn, err := grpc.NewClient(cfg.TradingSvcAddr, dialOpts...)
	if err != nil {
		log.Error("failed to connect to trading service", "error", err)
		os.Exit(1)
	}
	defer tradingConn.Close()

	rankingConn, err := grpc.NewClient(cfg.RankingSvcAddr, dialOpts...)
	if err != nil {
		log.Error("failed to connect to ranking service", "error", err)
		os.Exit(1)
	}
	defer rankingConn.Close()

	// ---------- gRPC service clients ----------
	authClient := authv1.NewAuthServiceClient(authConn)
	marketClient := marketv1.NewMarketServiceClient(marketConn)
	tradingClient := tradingv1.NewTradingServiceClient(tradingConn)
	rankingClient := rankingv1.NewRankingServiceClient(rankingConn)

	// ---------- HTTP handlers ----------
	authHandler := handler.NewAuthHandler(authClient)
	marketHandler := handler.NewMarketHandler(marketClient)
	orderHandler := handler.NewOrderHandler(tradingClient)
	rankingHandler := handler.NewRankingHandler(rankingClient)
	adminHandler := handler.NewAdminHandler(marketClient, authClient)

	// ---------- Auth middleware ----------
	authMW := middleware.AuthMiddleware(authClient)

	// ---------- Redis rate limiter ----------
	var rateLimiter middleware.RateLimiter

	redisClient := goredis.NewClient(&goredis.Options{
		Addr: cfg.RedisAddr,
	})

	// Verify Redis connectivity. If the ping fails, the gateway starts
	// without rate limiting so it is not hard-dependent on Redis.
	pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pingCancel()

	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		log.Warn("redis unavailable, rate limiting disabled", "addr", cfg.RedisAddr, "error", err)
	} else {
		rateLimiter = infraredis.NewRateLimiter(redisClient)
		log.Info("rate limiter enabled", "addr", cfg.RedisAddr)
	}

	// ---------- HTTP Router ----------
	router := internal.SetupRouter(serviceName, rateLimiter, authHandler, marketHandler, orderHandler, rankingHandler, adminHandler, authMW)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// ---------- Start ----------
	go func() {
		log.Info("gateway listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server error", "error", err)
			cancel()
		}
	}()

	// ---------- Graceful shutdown ----------
	<-ctx.Done()
	log.Info("shutting down gateway")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http server shutdown error", "error", err)
	}

	log.Info("gateway stopped")
}
