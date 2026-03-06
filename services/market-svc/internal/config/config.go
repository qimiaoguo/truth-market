package config

import "os"

// Config holds the market service configuration.
type Config struct {
	Port           string
	DatabaseURL    string
	RedisAddr      string
	OTelEndpoint   string
	TradingSvcAddr string
}

// Load reads configuration from environment variables, falling back to
// sensible defaults for local development.
func Load() (*Config, error) {
	return &Config{
		Port:           envOrDefault("PORT", "9002"),
		DatabaseURL:    envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/truthmarket?sslmode=disable"),
		RedisAddr:      envOrDefault("REDIS_ADDR", "localhost:6379"),
		OTelEndpoint:   envOrDefault("OTEL_ENDPOINT", "localhost:4317"),
		TradingSvcAddr: envOrDefault("TRADING_SVC_ADDR", "localhost:9003"),
	}, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
