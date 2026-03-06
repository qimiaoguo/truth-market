package repository

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// MarketFilter holds optional criteria for listing markets.
type MarketFilter struct {
	Status   *domain.MarketStatus
	Category *string
	Limit    int
	Offset   int
}

// MarketRepository defines persistence operations for prediction markets.
type MarketRepository interface {
	// Create inserts a new market record.
	Create(ctx context.Context, market *domain.Market) error

	// GetByID retrieves a market by its unique identifier.
	GetByID(ctx context.Context, id string) (*domain.Market, error)

	// Update persists changes to an existing market record.
	Update(ctx context.Context, market *domain.Market) error

	// List returns markets matching the given filter along with the total count.
	List(ctx context.Context, filter MarketFilter) ([]*domain.Market, int64, error)
}
