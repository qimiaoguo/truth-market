package redis_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	redisinfra "github.com/truthmarket/truth-market/infra/redis"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// --------------------------------------------------------------------------
// EventBus integration tests
// --------------------------------------------------------------------------

func TestEventBus_Publish_SendsToRedisChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	ctx := context.Background()
	bus := redisinfra.NewEventBus(testClient)
	defer bus.Close()

	topic := "test.publish.raw"

	// Use a native Redis subscription to verify that Publish actually sends
	// a message to the underlying Redis Pub/Sub channel.
	nativeSub := testClient.Subscribe(ctx, topic)
	defer nativeSub.Close()

	// Wait for the subscription to be active before publishing.
	// Redis subscriptions need a moment to be established.
	_, err := nativeSub.Receive(ctx)
	require.NoError(t, err, "native subscription should be confirmed")

	event := domain.DomainEvent{
		ID:        "evt-001",
		Type:      domain.EventTradeExecuted,
		Payload:   json.RawMessage(`{"price":"42.50"}`),
		Source:    "test",
		Timestamp: time.Now().UTC().Truncate(time.Second),
		TraceID:   "trace-abc",
		SpanID:    "span-def",
	}

	err = bus.Publish(ctx, topic, event)
	require.NoError(t, err, "Publish should not return an error")

	// Read from the native channel with a timeout.
	select {
	case msg := <-nativeSub.Channel():
		require.NotNil(t, msg, "should receive a message on the Redis channel")
		assert.Equal(t, topic, msg.Channel, "message should arrive on the correct channel")

		// Verify the payload is valid JSON containing our event ID.
		var received domain.DomainEvent
		err := json.Unmarshal([]byte(msg.Payload), &received)
		require.NoError(t, err, "message payload should be valid JSON")
		assert.Equal(t, "evt-001", received.ID, "event ID should match")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for message on native Redis subscription")
	}
}

func TestEventBus_Subscribe_ReceivesPublishedEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	ctx := context.Background()
	bus := redisinfra.NewEventBus(testClient)
	defer bus.Close()

	topic := "test.subscribe.receive"

	received := make(chan domain.DomainEvent, 1)

	err := bus.Subscribe(ctx, topic, func(ctx context.Context, e domain.DomainEvent) error {
		received <- e
		return nil
	})
	require.NoError(t, err, "Subscribe should not return an error")

	// Small delay to allow the subscription goroutine to start and register
	// with Redis before we publish.
	time.Sleep(100 * time.Millisecond)

	event := domain.DomainEvent{
		ID:        "evt-sub-001",
		Type:      domain.EventOrderPlaced,
		Payload:   json.RawMessage(`{"market_id":"mkt-1","side":"yes"}`),
		Source:    "trading-svc",
		Timestamp: time.Now().UTC().Truncate(time.Second),
		TraceID:   "trace-sub-001",
		SpanID:    "span-sub-001",
	}

	err = bus.Publish(ctx, topic, event)
	require.NoError(t, err, "Publish should not return an error")

	select {
	case got := <-received:
		assert.Equal(t, event.ID, got.ID, "event ID should match")
		assert.Equal(t, event.Type, got.Type, "event Type should match")
		assert.Equal(t, event.Source, got.Source, "event Source should match")
		assert.Equal(t, event.TraceID, got.TraceID, "event TraceID should match")
		assert.JSONEq(t, string(event.Payload), string(got.Payload), "event Payload should match")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for handler to receive event")
	}
}

func TestEventBus_Subscribe_MultipleTopics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	ctx := context.Background()
	bus := redisinfra.NewEventBus(testClient)
	defer bus.Close()

	topicA := "test.multi.topic-a"
	topicB := "test.multi.topic-b"

	receivedA := make(chan domain.DomainEvent, 5)
	receivedB := make(chan domain.DomainEvent, 5)

	err := bus.Subscribe(ctx, topicA, func(ctx context.Context, e domain.DomainEvent) error {
		receivedA <- e
		return nil
	})
	require.NoError(t, err)

	err = bus.Subscribe(ctx, topicB, func(ctx context.Context, e domain.DomainEvent) error {
		receivedB <- e
		return nil
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	eventA := domain.DomainEvent{
		ID:     "evt-topic-a",
		Type:   domain.EventMarketCreated,
		Source: "market-svc",
	}
	eventB := domain.DomainEvent{
		ID:     "evt-topic-b",
		Type:   domain.EventBalanceUpdated,
		Source: "trading-svc",
	}

	err = bus.Publish(ctx, topicA, eventA)
	require.NoError(t, err)
	err = bus.Publish(ctx, topicB, eventB)
	require.NoError(t, err)

	// Verify topic A handler receives only eventA.
	select {
	case got := <-receivedA:
		assert.Equal(t, "evt-topic-a", got.ID, "topic A handler should receive event A")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event on topic A")
	}

	// Verify topic B handler receives only eventB.
	select {
	case got := <-receivedB:
		assert.Equal(t, "evt-topic-b", got.ID, "topic B handler should receive event B")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event on topic B")
	}

	// Verify no cross-talk: topic A should NOT have received eventB.
	select {
	case unexpected := <-receivedA:
		t.Fatalf("topic A handler should not receive events from topic B, but got: %s", unexpected.ID)
	case <-time.After(500 * time.Millisecond):
		// Expected: no additional messages.
	}

	// Verify no cross-talk: topic B should NOT have received eventA.
	select {
	case unexpected := <-receivedB:
		t.Fatalf("topic B handler should not receive events from topic A, but got: %s", unexpected.ID)
	case <-time.After(500 * time.Millisecond):
		// Expected: no additional messages.
	}
}

func TestEventBus_Subscribe_MultipleHandlers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	ctx := context.Background()
	bus := redisinfra.NewEventBus(testClient)
	defer bus.Close()

	topic := "test.multi.handler"

	received1 := make(chan domain.DomainEvent, 1)
	received2 := make(chan domain.DomainEvent, 1)

	err := bus.Subscribe(ctx, topic, func(ctx context.Context, e domain.DomainEvent) error {
		received1 <- e
		return nil
	})
	require.NoError(t, err)

	err = bus.Subscribe(ctx, topic, func(ctx context.Context, e domain.DomainEvent) error {
		received2 <- e
		return nil
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	event := domain.DomainEvent{
		ID:     "evt-multi-handler",
		Type:   domain.EventOrderCancelled,
		Source: "trading-svc",
	}

	err = bus.Publish(ctx, topic, event)
	require.NoError(t, err)

	// Both handlers should receive the same event.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		select {
		case got := <-received1:
			assert.Equal(t, "evt-multi-handler", got.ID, "handler 1 should receive the event")
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for handler 1 to receive event")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case got := <-received2:
			assert.Equal(t, "evt-multi-handler", got.ID, "handler 2 should receive the event")
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for handler 2 to receive event")
		}
	}()

	wg.Wait()
}

func TestEventBus_Close_StopsSubscriptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	ctx := context.Background()
	bus := redisinfra.NewEventBus(testClient)

	topic := "test.close.stops"

	received := make(chan domain.DomainEvent, 10)

	err := bus.Subscribe(ctx, topic, func(ctx context.Context, e domain.DomainEvent) error {
		received <- e
		return nil
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Publish an event before closing to confirm the subscription works.
	preCloseEvent := domain.DomainEvent{
		ID:     "evt-before-close",
		Type:   domain.EventTradeExecuted,
		Source: "test",
	}
	err = bus.Publish(ctx, topic, preCloseEvent)
	require.NoError(t, err)

	select {
	case got := <-received:
		assert.Equal(t, "evt-before-close", got.ID, "should receive event before close")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pre-close event")
	}

	// Now close the event bus.
	err = bus.Close()
	require.NoError(t, err, "Close should not return an error")

	// Allow time for the subscription goroutine to terminate.
	time.Sleep(200 * time.Millisecond)

	// Publish another event after closing. We use a separate publisher
	// (the raw Redis client) because the bus is closed.
	postCloseEvent := domain.DomainEvent{
		ID:     "evt-after-close",
		Type:   domain.EventTradeExecuted,
		Source: "test",
	}
	data, err := json.Marshal(postCloseEvent)
	require.NoError(t, err)
	err = testClient.Publish(ctx, topic, data).Err()
	require.NoError(t, err, "raw Redis publish should succeed even after bus close")

	// The handler should NOT receive the post-close event.
	select {
	case unexpected := <-received:
		t.Fatalf("handler should not receive events after Close(), but got: %s", unexpected.ID)
	case <-time.After(1 * time.Second):
		// Expected: no message received after close.
	}
}

func TestEventBus_Publish_PreservesEventData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	ctx := context.Background()
	bus := redisinfra.NewEventBus(testClient)
	defer bus.Close()

	topic := "test.preserve.data"

	received := make(chan domain.DomainEvent, 1)

	err := bus.Subscribe(ctx, topic, func(ctx context.Context, e domain.DomainEvent) error {
		received <- e
		return nil
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Construct an event with every field populated, including a non-trivial
	// JSON payload, to verify full round-trip fidelity.
	ts := time.Date(2026, 3, 6, 12, 30, 45, 0, time.UTC)
	payload := json.RawMessage(`{"market_id":"mkt-xyz","outcome":"yes","price":"0.73","quantity":100}`)

	original := domain.DomainEvent{
		ID:        "evt-preserve-all-fields",
		Type:      domain.EventMarketResolved,
		Payload:   payload,
		Source:    "market-svc",
		Timestamp: ts,
		TraceID:   "trace-preserve-abc-123",
		SpanID:    "span-preserve-def-456",
	}

	err = bus.Publish(ctx, topic, original)
	require.NoError(t, err)

	select {
	case got := <-received:
		assert.Equal(t, original.ID, got.ID, "ID should survive round-trip")
		assert.Equal(t, original.Type, got.Type, "Type should survive round-trip")
		assert.Equal(t, original.Source, got.Source, "Source should survive round-trip")
		assert.Equal(t, original.TraceID, got.TraceID, "TraceID should survive round-trip")
		assert.Equal(t, original.SpanID, got.SpanID, "SpanID should survive round-trip")
		assert.True(t, original.Timestamp.Equal(got.Timestamp),
			"Timestamp should survive round-trip: want %v, got %v", original.Timestamp, got.Timestamp)
		assert.JSONEq(t, string(original.Payload), string(got.Payload),
			"Payload should survive round-trip")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for handler to receive event")
	}
}
