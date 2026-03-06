package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	q := r.Querier(ctx)

	// Build a batch INSERT with multiple value rows.
	const baseCols = 5
	sql := `INSERT INTO outcomes (id, market_id, label, index, is_winner) VALUES `

	args := make([]any, 0, len(outcomes)*baseCols)
	valueRows := make([]string, 0, len(outcomes))

	for i, o := range outcomes {
		offset := i * baseCols
		valueRows = append(valueRows, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d)",
			offset+1, offset+2, offset+3, offset+4, offset+5,
		))
		args = append(args, o.ID, o.MarketID, o.Label, o.Index, o.IsWinner)
	}

	sql += joinStrings(valueRows, ", ")

	_, err := q.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("postgres: create outcomes batch: %w", err)
	}

	return nil
}

func (r *OutcomeRepo) ListByMarket(ctx context.Context, marketID string) ([]*domain.Outcome, error) {
	q := r.Querier(ctx)

	rows, err := q.Query(ctx,
		`SELECT id, market_id, label, index, is_winner
		 FROM outcomes WHERE market_id = $1 ORDER BY index ASC`, marketID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list outcomes by market: %w", err)
	}
	defer rows.Close()

	var outcomes []*domain.Outcome
	for rows.Next() {
		o, err := scanOutcomeFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: list outcomes scan: %w", err)
		}
		outcomes = append(outcomes, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list outcomes rows: %w", err)
	}

	return outcomes, nil
}

func (r *OutcomeRepo) SetWinner(ctx context.Context, id string) error {
	q := r.Querier(ctx)

	tag, err := q.Exec(ctx,
		`UPDATE outcomes SET is_winner = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: set outcome winner: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: set outcome winner: %w", pgx.ErrNoRows)
	}

	return nil
}

// scanOutcomeFromRows scans a single outcome from pgx.Rows.
func scanOutcomeFromRows(rows pgx.Rows) (*domain.Outcome, error) {
	var o domain.Outcome
	err := rows.Scan(
		&o.ID,
		&o.MarketID,
		&o.Label,
		&o.Index,
		&o.IsWinner,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// joinStrings joins string slices with a separator.
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}
