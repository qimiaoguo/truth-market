// Package eventbus defines the interface for asynchronous, topic-based domain
// event delivery within the truth-market platform.
//
// The [EventBus] interface decouples event producers from consumers. The
// initial implementation targets Redis Pub/Sub, but the abstraction allows
// swapping to NATS, Kafka, or any other message broker without changing
// calling code.
//
// Usage:
//
//	bus.Publish(ctx, eventbus.TopicTradeExecuted, event)
//
//	bus.Subscribe(ctx, eventbus.TopicTradeExecuted, func(ctx context.Context, e domain.DomainEvent) error {
//	    // handle event
//	    return nil
//	})
package eventbus

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// EventBus defines the interface for publishing and subscribing to domain
// events. Implementations must be safe for concurrent use.
type EventBus interface {
	// Publish sends an event to all subscribers of the given topic. The
	// implementation should serialize the event and deliver it
	// asynchronously. An error is returned if the event cannot be enqueued.
	Publish(ctx context.Context, topic string, event domain.DomainEvent) error

	// Subscribe registers a handler that is invoked for each event received
	// on the specified topic. Multiple handlers may be registered for the
	// same topic. The subscription remains active until [Close] is called.
	Subscribe(ctx context.Context, topic string, handler EventHandler) error

	// Close gracefully shuts down the event bus, draining in-flight
	// messages and releasing underlying resources.
	Close() error
}

// EventHandler is a callback function that processes a single domain event.
// Returning a non-nil error signals that the event could not be handled and
// may need to be retried (retry semantics depend on the implementation).
type EventHandler func(ctx context.Context, event domain.DomainEvent) error

// Topic constants define the well-known event topics used across services.
// Each topic corresponds to a specific domain event type.
const (
	// TopicTradeExecuted is published when a trade is matched and executed.
	TopicTradeExecuted = "trade.executed"

	// TopicOrderPlaced is published when a new order is submitted to the
	// order book.
	TopicOrderPlaced = "order.placed"

	// TopicOrderCancelled is published when an existing order is cancelled.
	TopicOrderCancelled = "order.cancelled"

	// TopicMarketCreated is published when a new prediction market is
	// created.
	TopicMarketCreated = "market.created"

	// TopicMarketResolved is published when a market outcome is determined
	// and the market transitions to the resolved state.
	TopicMarketResolved = "market.resolved"

	// TopicBalanceUpdated is published when a user's account balance
	// changes as a result of trading activity or administrative action.
	TopicBalanceUpdated = "balance.updated"
)
