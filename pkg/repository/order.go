package repository

import (
	"context"

	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// OrderRepository defines persistence operations for trading orders.
type OrderRepository interface {
	// Create inserts a new order record.
	Create(ctx context.Context, order *domain.Order) error

	// GetByID retrieves an order by its unique identifier.
	GetByID(ctx context.Context, id string) (*domain.Order, error)

	// UpdateStatus changes the status of an order and records the filled amount.
	UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, filled decimal.Decimal) error

	// ListOpenByMarket returns all open (unfilled) orders for a given market.
	ListOpenByMarket(ctx context.Context, marketID string) ([]*domain.Order, error)

	// ListByUser returns paginated orders for a user along with the total count.
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Order, int64, error)

	// CancelAllByMarket cancels every open order in the specified market and
	// returns the number of orders affected.
	CancelAllByMarket(ctx context.Context, marketID string) (int64, error)
}
