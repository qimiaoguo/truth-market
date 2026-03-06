package repository

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// OutcomeRepository defines persistence operations for market outcomes.
type OutcomeRepository interface {
	// CreateBatch inserts multiple outcome records in a single operation.
	CreateBatch(ctx context.Context, outcomes []*domain.Outcome) error

	// ListByMarket returns all outcomes associated with the given market.
	ListByMarket(ctx context.Context, marketID string) ([]*domain.Outcome, error)

	// SetWinner marks the outcome identified by id as the winning outcome.
	SetWinner(ctx context.Context, id string) error
}
