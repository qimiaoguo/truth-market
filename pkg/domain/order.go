package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// OrderSide indicates whether an order is buying or selling outcome shares.
type OrderSide string

const (
	// OrderSideBuy represents an order to purchase outcome shares.
	OrderSideBuy OrderSide = "buy"
	// OrderSideSell represents an order to sell outcome shares.
	OrderSideSell OrderSide = "sell"
)

// String returns the string representation of an OrderSide.
func (s OrderSide) String() string {
	return string(s)
}

// OrderStatus represents the current state of an order in the matching engine.
type OrderStatus string

const (
	// OrderStatusOpen indicates the order is on the book and waiting to be matched.
	OrderStatusOpen OrderStatus = "open"
	// OrderStatusPartial indicates the order has been partially filled.
	OrderStatusPartial OrderStatus = "partially_filled"
	// OrderStatusFilled indicates the order has been completely filled.
	OrderStatusFilled OrderStatus = "filled"
	// OrderStatusCancelled indicates the order has been cancelled by the user or system.
	OrderStatusCancelled OrderStatus = "cancelled"
)

// String returns the string representation of an OrderStatus.
func (s OrderStatus) String() string {
	return string(s)
}

// Order represents a limit order placed by a user on a specific market outcome.
// Orders are matched by the engine to produce trades.
type Order struct {
	ID        string
	UserID    string
	MarketID  string
	OutcomeID string
	Side      OrderSide
	Price     decimal.Decimal
	Quantity  decimal.Decimal
	FilledQty decimal.Decimal
	Status    OrderStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
