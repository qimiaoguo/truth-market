package domain

import "time"

// MarketStatus represents the lifecycle state of a prediction market.
type MarketStatus string

const (
	// MarketStatusDraft indicates the market has been created but is not yet open for trading.
	MarketStatusDraft MarketStatus = "draft"
	// MarketStatusOpen indicates the market is actively accepting orders.
	MarketStatusOpen MarketStatus = "open"
	// MarketStatusClosed indicates the market is no longer accepting new orders.
	MarketStatusClosed MarketStatus = "closed"
	// MarketStatusResolved indicates the market outcome has been determined.
	MarketStatusResolved MarketStatus = "resolved"
	// MarketStatusCancelled indicates the market has been cancelled and positions should be unwound.
	MarketStatusCancelled MarketStatus = "cancelled"
)

// String returns the string representation of a MarketStatus.
func (s MarketStatus) String() string {
	return string(s)
}

// MarketType defines the structure of outcomes in a prediction market.
type MarketType string

const (
	// MarketTypeBinary represents a market with exactly two outcomes (yes/no).
	MarketTypeBinary MarketType = "binary"
	// MarketTypeMulti represents a market with more than two possible outcomes.
	MarketTypeMulti MarketType = "multi"
)

// String returns the string representation of a MarketType.
func (t MarketType) String() string {
	return string(t)
}

// Market represents a prediction market where participants trade on the likelihood
// of future events. Each market has one or more outcomes that users can buy or sell shares in.
type Market struct {
	ID                string
	Title             string
	Description       string
	Category          string
	MarketType        MarketType
	Status            MarketStatus
	CreatorID         string
	ResolvedOutcomeID *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ClosesAt          *time.Time
}

// Outcome represents a possible result of a prediction market.
// For binary markets there are exactly two outcomes; for multi markets there can be many.
type Outcome struct {
	ID       string
	MarketID string
	Label    string
	Index    int
	IsWinner bool
}
