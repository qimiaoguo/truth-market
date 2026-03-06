package repository

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// PositionRepository defines persistence operations for user positions in
// market outcomes.
type PositionRepository interface {
	// Upsert creates or updates a position record. If a position already exists
	// for the same user and outcome, it is updated; otherwise a new record is
	// inserted.
	Upsert(ctx context.Context, position *domain.Position) error

	// GetByUserAndOutcome retrieves the position for a specific user and outcome
	// combination.
	GetByUserAndOutcome(ctx context.Context, userID, outcomeID string) (*domain.Position, error)

	// ListByUser returns all positions held by the specified user.
	ListByUser(ctx context.Context, userID string) ([]*domain.Position, error)

	// ListByMarket returns all positions across all outcomes in the specified
	// market.
	ListByMarket(ctx context.Context, marketID string) ([]*domain.Position, error)
}
