package domain

import (
	"encoding/json"
	"time"
)

// Event type constants identify the kind of domain event for routing and processing.
const (
	// EventTradeExecuted is emitted when a trade is matched and executed.
	EventTradeExecuted = "trade.executed"
	// EventOrderPlaced is emitted when a new order is submitted to the book.
	EventOrderPlaced = "order.placed"
	// EventOrderCancelled is emitted when an order is cancelled.
	EventOrderCancelled = "order.cancelled"
	// EventMarketCreated is emitted when a new prediction market is created.
	EventMarketCreated = "market.created"
	// EventMarketResolved is emitted when a market outcome is determined.
	EventMarketResolved = "market.resolved"
	// EventBalanceUpdated is emitted when a user's balance changes.
	EventBalanceUpdated = "balance.updated"
)

// DomainEvent represents an event that has occurred within the domain.
// Events are used for inter-service communication and audit logging.
// The Payload field contains event-specific data serialized as JSON.
type DomainEvent struct {
	ID        string
	Type      string
	Payload   json.RawMessage
	Source    string
	Timestamp time.Time
	TraceID   string
	SpanID    string
}
