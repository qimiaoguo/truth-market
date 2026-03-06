package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/truthmarket/truth-market/infra/testutil"
)

// testPool is the shared connection pool initialised once by TestMain and used
// by every integration test in this package.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	dsn, cleanup, err := testutil.PostgresContainer(ctx)
	if err != nil {
		panic("failed to start postgres container: " + err.Error())
	}
	defer cleanup()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic("failed to create pool: " + err.Error())
	}
	defer pool.Close()

	// Apply the users migration.
	migrationSQL := `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_address VARCHAR(42) UNIQUE,
    user_type VARCHAR(10) NOT NULL DEFAULT 'human' CHECK (user_type IN ('human', 'agent')),
    balance DECIMAL(20, 8) NOT NULL DEFAULT 1000.00000000,
    locked_balance DECIMAL(20, 8) NOT NULL DEFAULT 0.00000000,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_wallet_address ON users(wallet_address);
CREATE INDEX IF NOT EXISTS idx_users_user_type ON users(user_type);
`
	if _, err := pool.Exec(ctx, migrationSQL); err != nil {
		panic("failed to run migration: " + err.Error())
	}

	testPool = pool
	os.Exit(m.Run())
}

// truncateUsers removes all rows so each test starts with a clean table.
func truncateUsers(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE TABLE users CASCADE")
	require.NoError(t, err)
}
