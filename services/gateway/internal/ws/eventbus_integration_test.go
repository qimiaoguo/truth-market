package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/eventbus"
)

// mockEventBus is a simple in-process eventbus for integration testing.
// It allows Publish to fan-out to all Subscribe handlers without Redis.
type mockEventBus struct {
	handlers map[string][]eventbus.EventHandler
}

func newMockEventBus() *mockEventBus {
	return &mockEventBus{
		handlers: make(map[string][]eventbus.EventHandler),
	}
}

func (m *mockEventBus) Publish(ctx context.Context, topic string, event domain.DomainEvent) error {
	for _, h := range m.handlers[topic] {
		_ = h(ctx, event)
	}
	return nil
}

func (m *mockEventBus) Subscribe(_ context.Context, topic string, handler eventbus.EventHandler) error {
	m.handlers[topic] = append(m.handlers[topic], handler)
	return nil
}

func (m *mockEventBus) Close() error {
	m.handlers = make(map[string][]eventbus.EventHandler)
	return nil
}

// ---------------------------------------------------------------------------
// Tests: EventBus → Hub integration
// ---------------------------------------------------------------------------

// TestEventBus_TradeEvent_BroadcastsToMarketChannel verifies that when a
// trade.executed event is published on the event bus, clients subscribed to
// the corresponding market channel receive the event via their Send channel.
func TestEventBus_TradeEvent_BroadcastsToMarketChannel(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	bus := newMockEventBus()

	// Wire up the EventBus → Hub bridge.
	ConnectEventBus(hub, bus)

	// Create and register a client subscribed to market-1.
	client := &Client{
		ID:     "client-1",
		UserID: "user-1",
		Send:   make(chan []byte, 256),
		hub:    hub,
	}
	hub.Register(client)
	// Allow registration to be processed.
	time.Sleep(10 * time.Millisecond)

	hub.SubscribeMarket(client, "market-1")
	time.Sleep(10 * time.Millisecond)

	// Publish a trade event for market-1.
	tradePayload, _ := json.Marshal(map[string]interface{}{
		"trade_id":  "trade-100",
		"market_id": "market-1",
		"price":     "0.55",
		"quantity":  "10",
	})
	tradeEvent := domain.DomainEvent{
		ID:        "evt-1",
		Type:      domain.EventTradeExecuted,
		Payload:   tradePayload,
		Source:    "trading-svc",
		Timestamp: time.Now(),
	}

	err := bus.Publish(context.Background(), eventbus.TopicTradeExecuted, tradeEvent)
	require.NoError(t, err)

	// Client should receive the trade event.
	select {
	case msg := <-client.Send:
		var wsMsg WSMessage
		err := json.Unmarshal(msg, &wsMsg)
		require.NoError(t, err)
		assert.Equal(t, "event", wsMsg.Type)
		assert.Contains(t, string(wsMsg.Payload), "trade-100")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for trade event on client Send channel")
	}
}

// TestEventBus_OrderEvent_BroadcastsToUserChannel verifies that when an
// order.placed event is published, only the private user channel of the
// order owner receives the event.
func TestEventBus_OrderEvent_BroadcastsToUserChannel(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	bus := newMockEventBus()
	ConnectEventBus(hub, bus)

	// Create a client subscribed to user-1's private channel.
	client1 := &Client{
		ID:     "client-1",
		UserID: "user-1",
		Send:   make(chan []byte, 256),
		hub:    hub,
	}
	client2 := &Client{
		ID:     "client-2",
		UserID: "user-2",
		Send:   make(chan []byte, 256),
		hub:    hub,
	}

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(10 * time.Millisecond)

	hub.SubscribeUser(client1, "user-1")
	hub.SubscribeUser(client2, "user-2")
	time.Sleep(10 * time.Millisecond)

	// Publish an order event for user-1.
	orderPayload, _ := json.Marshal(map[string]interface{}{
		"order_id": "order-50",
		"user_id":  "user-1",
		"side":     "buy",
		"price":    "0.45",
	})
	orderEvent := domain.DomainEvent{
		ID:        "evt-2",
		Type:      domain.EventOrderPlaced,
		Payload:   orderPayload,
		Source:    "trading-svc",
		Timestamp: time.Now(),
	}

	err := bus.Publish(context.Background(), eventbus.TopicOrderPlaced, orderEvent)
	require.NoError(t, err)

	// Client 1 (user-1) should receive the event.
	select {
	case msg := <-client1.Send:
		var wsMsg WSMessage
		err := json.Unmarshal(msg, &wsMsg)
		require.NoError(t, err)
		assert.Equal(t, "event", wsMsg.Type)
		assert.Contains(t, string(wsMsg.Payload), "order-50")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for order event on client-1 Send channel")
	}

	// Client 2 (user-2) should NOT receive the event.
	select {
	case msg := <-client2.Send:
		t.Fatalf("client-2 should not have received event, but got: %s", msg)
	case <-time.After(100 * time.Millisecond):
		// Expected: no message for client-2.
	}
}

// TestEventBus_PriceChange_BroadcastsToMarketChannel verifies that market
// price change events (derived from trade events) are broadcast to all
// clients subscribed to the corresponding market channel.
func TestEventBus_PriceChange_BroadcastsToMarketChannel(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	bus := newMockEventBus()
	ConnectEventBus(hub, bus)

	// Two clients subscribed to market-1.
	client1 := &Client{
		ID:     "client-1",
		UserID: "user-1",
		Send:   make(chan []byte, 256),
		hub:    hub,
	}
	client2 := &Client{
		ID:     "client-2",
		UserID: "user-2",
		Send:   make(chan []byte, 256),
		hub:    hub,
	}

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(10 * time.Millisecond)

	hub.SubscribeMarket(client1, "market-1")
	hub.SubscribeMarket(client2, "market-1")
	time.Sleep(10 * time.Millisecond)

	// Publish a trade event (which implies a price change for market-1).
	tradePayload, _ := json.Marshal(map[string]interface{}{
		"trade_id":  "trade-200",
		"market_id": "market-1",
		"price":     "0.70",
		"quantity":  "5",
	})
	tradeEvent := domain.DomainEvent{
		ID:        "evt-3",
		Type:      domain.EventTradeExecuted,
		Payload:   tradePayload,
		Source:    "trading-svc",
		Timestamp: time.Now(),
	}

	err := bus.Publish(context.Background(), eventbus.TopicTradeExecuted, tradeEvent)
	require.NoError(t, err)

	// Both clients should receive the trade event.
	for _, c := range []*Client{client1, client2} {
		select {
		case msg := <-c.Send:
			var wsMsg WSMessage
			err := json.Unmarshal(msg, &wsMsg)
			require.NoError(t, err)
			assert.Equal(t, "event", wsMsg.Type)
			assert.Contains(t, string(wsMsg.Payload), "trade-200")
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for trade event on client %s", c.ID)
		}
	}
}
