package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// txKey is the context key used to propagate a transaction handle.
type txKey struct{}

// PgTxManager implements repository.TxManager using pgx transactions.
type PgTxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a new PgTxManager backed by the given connection pool.
func NewTxManager(pool *pgxpool.Pool) *PgTxManager {
	return &PgTxManager{pool: pool}
}

// WithTx executes fn within a database transaction. The transaction is committed
// if fn returns nil; otherwise it is rolled back. The pgx.Tx handle is stored in
// the context so that any repository using Querier(ctx) will participate in the
// same transaction.
func (m *PgTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() {
		// Rollback is a no-op if the tx has already been committed.
		_ = tx.Rollback(ctx)
	}()

	txCtx := context.WithValue(ctx, txKey{}, tx)

	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit tx: %w", err)
	}

	return nil
}

// txFromCtx extracts a pgx.Tx from the context if one exists.
func txFromCtx(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
