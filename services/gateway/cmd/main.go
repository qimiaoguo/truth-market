package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/truthmarket/truth-market/pkg/logger"
	"github.com/truthmarket/truth-market/pkg/otel"
	otelmw "github.com/truthmarket/truth-market/pkg/otel/middleware"
	"github.com/truthmarket/truth-market/services/gateway/internal"
	"github.com/truthmarket/truth-market/services/gateway/internal/config"
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

	// Connections will be used once service-specific gRPC clients are wired.
	_ = authConn
	_ = marketConn
	_ = tradingConn
	_ = rankingConn

	// ---------- HTTP Router ----------
	router := internal.SetupRouter(serviceName)

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
