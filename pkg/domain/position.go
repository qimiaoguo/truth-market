package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Position represents a user's current holding in a specific market outcome.
// The quantity and average price are updated as trades are executed.
type Position struct {
	ID        string
	UserID    string
	MarketID  string
	OutcomeID string
	Quantity  decimal.Decimal
	AvgPrice  decimal.Decimal
	UpdatedAt time.Time
}

// MintTransaction records the creation of a complete set of outcome shares for a market.
// When a user mints shares, they pay the cost and receive an equal quantity of each outcome token.
type MintTransaction struct {
	ID        string
	UserID    string
	MarketID  string
	Quantity  decimal.Decimal
	Cost      decimal.Decimal
	CreatedAt time.Time
}
