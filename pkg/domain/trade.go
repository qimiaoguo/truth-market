package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Trade represents a completed transaction between two orders in the matching engine.
// Each trade records the maker (resting order) and taker (incoming order) sides.
type Trade struct {
	ID           string
	MarketID     string
	OutcomeID    string
	MakerOrderID string
	TakerOrderID string
	MakerUserID  string
	TakerUserID  string
	Price        decimal.Decimal
	Quantity     decimal.Decimal
	CreatedAt    time.Time
}
