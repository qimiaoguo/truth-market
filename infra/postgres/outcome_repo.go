package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/truthmarket/truth-market/infra/postgres/sqlcgen"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// OutcomeRepo implements repository.OutcomeRepository using PostgreSQL.
type OutcomeRepo struct {
	BaseRepo
}

// NewOutcomeRepo creates a new OutcomeRepo.
func NewOutcomeRepo(pool *pgxpool.Pool) *OutcomeRepo {
	return &OutcomeRepo{BaseRepo: BaseRepo{pool: pool}}
}

// compile-time interface check
var _ repository.OutcomeRepository = (*OutcomeRepo)(nil)

func (r *OutcomeRepo) CreateBatch(ctx context.Context, outcomes []*domain.Outcome) error {
	if len(outcomes) == 0 {
		return nil
	}

	params := make([]sqlcgen.CreateOutcomeBatchParams, len(outcomes))
	for i, o := range outcomes {
		params[i] = sqlcgen.CreateOutcomeBatchParams{
			ID:       o.ID,
			MarketID: o.MarketID,
			Label:    o.Label,
			Index:    int32(o.Index),
			IsWinner: o.IsWinner,
		}
	}

	_, err := r.Q(ctx).CreateOutcomeBatch(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: create outcomes batch: %w", err)
	}
	return nil
}

func (r *OutcomeRepo) ListByMarket(ctx context.Context, marketID string) ([]*domain.Outcome, error) {
	rows, err := r.Q(ctx).ListOutcomesByMarket(ctx, marketID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list outcomes by market: %w", err)
	}

	outcomes := make([]*domain.Outcome, len(rows))
	for i, row := range rows {
		outcomes[i] = &domain.Outcome{
			ID:       row.ID,
			MarketID: row.MarketID,
			Label:    row.Label,
			Index:    int(row.Index),
			IsWinner: row.IsWinner,
		}
	}
	return outcomes, nil
}

func (r *OutcomeRepo) SetWinner(ctx context.Context, id string) error {
	n, err := r.Q(ctx).SetOutcomeWinner(ctx, id)
	if err != nil {
		return fmt.Errorf("postgres: set outcome winner: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("postgres: set outcome winner: %w", pgx.ErrNoRows)
	}
	return nil
}
