package repository

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// TradeRepository defines persistence operations for executed trades.
type TradeRepository interface {
	// Create inserts a new trade record.
	Create(ctx context.Context, trade *domain.Trade) error

	// ListByMarket returns paginated trades for a market along with the total count.
	ListByMarket(ctx context.Context, marketID string, limit, offset int) ([]*domain.Trade, int64, error)

	// ListByUser returns paginated trades for a user along with the total count.
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Trade, int64, error)

	// CreateMintTx records a mint transaction associated with a trade.
	CreateMintTx(ctx context.Context, tx *domain.MintTransaction) error
}
