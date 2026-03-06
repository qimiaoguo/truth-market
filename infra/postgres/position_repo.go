package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// PositionRepo implements repository.PositionRepository using PostgreSQL.
type PositionRepo struct {
	BaseRepo
}

// NewPositionRepo creates a new PositionRepo.
func NewPositionRepo(pool *pgxpool.Pool) *PositionRepo {
	return &PositionRepo{BaseRepo: BaseRepo{pool: pool}}
}

// compile-time interface check
var _ repository.PositionRepository = (*PositionRepo)(nil)

func (r *PositionRepo) Upsert(ctx context.Context, position *domain.Position) error {
	q := r.Querier(ctx)

	_, err := q.Exec(ctx,
		`INSERT INTO positions (id, user_id, market_id, outcome_id, quantity, avg_price, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (user_id, outcome_id) DO UPDATE SET
			quantity = EXCLUDED.quantity,
			avg_price = EXCLUDED.avg_price,
			updated_at = EXCLUDED.updated_at`,
		position.ID,
		position.UserID,
		position.MarketID,
		position.OutcomeID,
		position.Quantity,
		position.AvgPrice,
		position.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: upsert position: %w", err)
	}

	return nil
}

func (r *PositionRepo) GetByUserAndOutcome(ctx context.Context, userID, outcomeID string) (*domain.Position, error) {
	q := r.Querier(ctx)

	row := q.QueryRow(ctx,
		`SELECT id, user_id, market_id, outcome_id, quantity, avg_price, updated_at
		 FROM positions WHERE user_id = $1 AND outcome_id = $2`, userID, outcomeID)

	p, err := scanPosition(row)
	if err != nil {
		return nil, fmt.Errorf("postgres: get position by user and outcome: %w", err)
	}

	return p, nil
}

func (r *PositionRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Position, error) {
	q := r.Querier(ctx)

	rows, err := q.Query(ctx,
		`SELECT id, user_id, market_id, outcome_id, quantity, avg_price, updated_at
		 FROM positions WHERE user_id = $1
		 ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list positions by user: %w", err)
	}
	defer rows.Close()

	var positions []*domain.Position
	for rows.Next() {
		p, err := scanPositionFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: list positions by user scan: %w", err)
		}
		positions = append(positions, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list positions by user rows: %w", err)
	}

	return positions, nil
}

func (r *PositionRepo) ListByMarket(ctx context.Context, marketID string) ([]*domain.Position, error) {
	q := r.Querier(ctx)

	rows, err := q.Query(ctx,
		`SELECT id, user_id, market_id, outcome_id, quantity, avg_price, updated_at
		 FROM positions WHERE market_id = $1
		 ORDER BY updated_at DESC`, marketID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list positions by market: %w", err)
	}
	defer rows.Close()

	var positions []*domain.Position
	for rows.Next() {
		p, err := scanPositionFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: list positions by market scan: %w", err)
		}
		positions = append(positions, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list positions by market rows: %w", err)
	}

	return positions, nil
}

// scanPosition scans a single position from pgx.Row.
func scanPosition(row pgx.Row) (*domain.Position, error) {
	var p domain.Position

	err := row.Scan(
		&p.ID,
		&p.UserID,
		&p.MarketID,
		&p.OutcomeID,
		&p.Quantity,
		&p.AvgPrice,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// scanPositionFromRows scans a single position from pgx.Rows.
func scanPositionFromRows(rows pgx.Rows) (*domain.Position, error) {
	var p domain.Position

	err := rows.Scan(
		&p.ID,
		&p.UserID,
		&p.MarketID,
		&p.OutcomeID,
		&p.Quantity,
		&p.AvgPrice,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &p, nil
}
