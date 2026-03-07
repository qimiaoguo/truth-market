package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/truthmarket/truth-market/infra/postgres/sqlcgen"
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
	err := r.Q(ctx).UpsertPosition(ctx, sqlcgen.UpsertPositionParams{
		ID:        position.ID,
		UserID:    position.UserID,
		MarketID:  position.MarketID,
		OutcomeID: position.OutcomeID,
		Quantity:  position.Quantity,
		AvgPrice:  position.AvgPrice,
		UpdatedAt: tstz(position.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("postgres: upsert position: %w", err)
	}
	return nil
}

func (r *PositionRepo) GetByUserAndOutcome(ctx context.Context, userID, outcomeID string) (*domain.Position, error) {
	row, err := r.Q(ctx).GetPositionByUserAndOutcome(ctx, sqlcgen.GetPositionByUserAndOutcomeParams{
		UserID:    userID,
		OutcomeID: outcomeID,
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: get position by user and outcome: %w", err)
	}
	return positionFromModel(row), nil
}

func (r *PositionRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Position, error) {
	rows, err := r.Q(ctx).ListPositionsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list positions by user: %w", err)
	}
	return positionsFromModels(rows), nil
}

func (r *PositionRepo) ListByMarket(ctx context.Context, marketID string) ([]*domain.Position, error) {
	rows, err := r.Q(ctx).ListPositionsByMarket(ctx, marketID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list positions by market: %w", err)
	}
	return positionsFromModels(rows), nil
}

func positionFromModel(r sqlcgen.Position) *domain.Position {
	return &domain.Position{
		ID:        r.ID,
		UserID:    r.UserID,
		MarketID:  r.MarketID,
		OutcomeID: r.OutcomeID,
		Quantity:  r.Quantity,
		AvgPrice:  r.AvgPrice,
		UpdatedAt: r.UpdatedAt.Time,
	}
}

func positionsFromModels(rows []sqlcgen.Position) []*domain.Position {
	positions := make([]*domain.Position, len(rows))
	for i, row := range rows {
		positions[i] = positionFromModel(row)
	}
	return positions
}
