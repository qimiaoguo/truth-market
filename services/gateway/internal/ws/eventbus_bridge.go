package ws

import (
	"context"
	"encoding/json"

	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/eventbus"
)

// ConnectEventBus wires an EventBus to a Hub so that domain events published
// on the bus are automatically forwarded to the appropriate WebSocket
// subscription channels.
//
// Currently supported routing:
//   - trade.executed -> market:<market_id>  (broadcast to market subscribers)
//   - order.placed   -> user:<user_id>      (broadcast to user subscribers)
func ConnectEventBus(hub *Hub, bus eventbus.EventBus) {
	ctx := context.Background()

	// Trade events are broadcast to everyone watching the market.
	_ = bus.Subscribe(ctx, eventbus.TopicTradeExecuted, func(ctx context.Context, e domain.DomainEvent) error {
		var data map[string]interface{}
		if err := json.Unmarshal(e.Payload, &data); err != nil {
			return nil
		}
		marketID, _ := data["market_id"].(string)
		if marketID == "" {
			return nil
		}
		msg := NewEventMessage("market:"+marketID, e.Payload)
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return nil
		}
		hub.BroadcastMarket(marketID, msgBytes)
		return nil
	})

	// Order events are sent only to the owning user's private channel.
	_ = bus.Subscribe(ctx, eventbus.TopicOrderPlaced, func(ctx context.Context, e domain.DomainEvent) error {
		var data map[string]interface{}
		if err := json.Unmarshal(e.Payload, &data); err != nil {
			return nil
		}
		userID, _ := data["user_id"].(string)
		if userID == "" {
			return nil
		}
		msg := NewEventMessage("user:"+userID, e.Payload)
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return nil
		}
		hub.BroadcastUser(userID, msgBytes)
		return nil
	})
}
