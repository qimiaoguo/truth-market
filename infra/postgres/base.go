package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier abstracts pgxpool.Pool and pgx.Tx so that repository methods can
// operate transparently inside or outside of a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// BaseRepo provides shared infrastructure for all PostgreSQL repository
// implementations. Concrete repos embed this struct and call Querier(ctx) to
// obtain a database handle that participates in the current transaction when one
// is active.
type BaseRepo struct {
	pool *pgxpool.Pool
}

// Querier returns the pgx.Tx stored in ctx (if running inside WithTx), or falls
// back to the connection pool for standalone queries.
func (r *BaseRepo) Querier(ctx context.Context) Querier {
	if tx, ok := txFromCtx(ctx); ok {
		return tx
	}
	return r.pool
}
