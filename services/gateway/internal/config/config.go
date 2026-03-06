package config

import "os"

// Config holds the gateway service configuration.
type Config struct {
	Port           string
	AuthSvcAddr    string
	MarketSvcAddr  string
	TradingSvcAddr string
	RankingSvcAddr string
	RedisAddr      string
	OTelEndpoint   string
	JWTSecret      string
}

// Load reads configuration from environment variables, falling back to
// sensible defaults for local development.
func Load() (*Config, error) {
	return &Config{
		Port:           envOrDefault("PORT", "8080"),
		AuthSvcAddr:    envOrDefault("AUTH_SVC_ADDR", "localhost:9001"),
		MarketSvcAddr:  envOrDefault("MARKET_SVC_ADDR", "localhost:9002"),
		TradingSvcAddr: envOrDefault("TRADING_SVC_ADDR", "localhost:9003"),
		RankingSvcAddr: envOrDefault("RANKING_SVC_ADDR", "localhost:9004"),
		RedisAddr:      envOrDefault("REDIS_ADDR", "localhost:6379"),
		OTelEndpoint:   envOrDefault("OTEL_ENDPOINT", "localhost:4317"),
		JWTSecret:      envOrDefault("JWT_SECRET", "dev-secret"),
	}, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
