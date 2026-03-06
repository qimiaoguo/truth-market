// Package testutil provides shared test infrastructure for the truth-market
// platform. It includes helpers for starting throwaway PostgreSQL and Redis
// containers via testcontainers-go, factory functions for building domain
// objects with sensible defaults, and custom test assertions for common
// comparison patterns.
package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage = "postgres:16-alpine"
	redisImage    = "redis:7-alpine"

	defaultDBName   = "truth_market_test"
	defaultDBUser   = "testuser"
	defaultDBPass   = "testpass"
	defaultStartup  = 30 * time.Second
)

// PostgresContainer starts a disposable PostgreSQL container and returns a DSN
// that can be passed directly to pgx or database/sql. The returned cleanup
// function terminates the container; callers should defer it immediately.
func PostgresContainer(ctx context.Context) (dsn string, cleanup func(), err error) {
	ctr, err := postgres.Run(ctx,
		postgresImage,
		postgres.WithDatabase(defaultDBName),
		postgres.WithUsername(defaultDBUser),
		postgres.WithPassword(defaultDBPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(defaultStartup),
		),
	)
	if err != nil {
		return "", nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		return "", nil, fmt.Errorf("get postgres connection string: %w", err)
	}

	cleanup = func() {
		_ = ctr.Terminate(context.Background())
	}

	return connStr, cleanup, nil
}

// RedisContainer starts a disposable Redis container and returns the host:port
// address suitable for go-redis. The returned cleanup function terminates the
// container; callers should defer it immediately.
func RedisContainer(ctx context.Context) (addr string, cleanup func(), err error) {
	ctr, err := redis.Run(ctx,
		redisImage,
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(defaultStartup),
		),
	)
	if err != nil {
		return "", nil, fmt.Errorf("start redis container: %w", err)
	}

	connStr, err := ctr.ConnectionString(ctx)
	if err != nil {
		_ = ctr.Terminate(ctx)
		return "", nil, fmt.Errorf("get redis connection string: %w", err)
	}

	cleanup = func() {
		_ = ctr.Terminate(context.Background())
	}

	return connStr, cleanup, nil
}
