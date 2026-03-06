package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	goredis "github.com/redis/go-redis/v9"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/eventbus"
)

// RedisEventBus implements eventbus.EventBus using Redis Pub/Sub for
// asynchronous domain event delivery.
type RedisEventBus struct {
	client *goredis.Client
	subs   []*goredis.PubSub
	mu     sync.Mutex
}

// NewEventBus returns a new RedisEventBus backed by the given Redis client.
func NewEventBus(client *goredis.Client) *RedisEventBus {
	return &RedisEventBus{
		client: client,
	}
}

// Publish serializes the domain event as JSON and publishes it to the named
// Redis Pub/Sub channel.
func (b *RedisEventBus) Publish(ctx context.Context, topic string, event domain.DomainEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return b.client.Publish(ctx, topic, data).Err()
}

// Subscribe registers a handler that is invoked for each event received on the
// specified Redis Pub/Sub channel. The subscription runs in a background
// goroutine and remains active until Close is called.
func (b *RedisEventBus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	sub := b.client.Subscribe(ctx, topic)

	b.mu.Lock()
	b.subs = append(b.subs, sub)
	b.mu.Unlock()

	go func() {
		ch := sub.Channel()
		for msg := range ch {
			var event domain.DomainEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}
			_ = handler(ctx, event)
		}
	}()

	return nil
}

// Close gracefully shuts down all active subscriptions.
func (b *RedisEventBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var firstErr error
	for _, sub := range b.subs {
		if err := sub.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	b.subs = nil
	return firstErr
}
