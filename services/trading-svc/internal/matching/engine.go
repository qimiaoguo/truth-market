package matching

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// EventType identifies the kind of event emitted by the matching engine.
type EventType string

const (
	// EventTrade is emitted when a trade is executed.
	EventTrade EventType = "trade"
	// EventBookUpdate is emitted when the orderbook state changes (e.g., a resting order is added).
	EventBookUpdate EventType = "book_update"
)

// Event represents a notification emitted by the matching engine.
type Event struct {
	Type      EventType
	MarketID  string
	OutcomeID string
	Trade     *domain.Trade
	Orderbook *Orderbook
}

// MatchResult contains the outcome of placing an order.
type MatchResult struct {
	Trades  []*domain.Trade
	Resting *domain.Order
}

var (
	errInvalidPrice    = errors.New("price must be between 0.01 and 0.99 inclusive")
	errInvalidQuantity = errors.New("quantity must be greater than 0")
	errOrderNotFound   = errors.New("order not found")
)

var (
	minPrice = decimal.NewFromFloat(0.01)
	maxPrice = decimal.NewFromFloat(0.99)
)

// Engine is the central matching engine that manages per-outcome orderbooks
// for a single market.
type Engine struct {
	mu          sync.RWMutex
	marketID    string
	orderbooks  map[string]*Orderbook
	subscribers []chan<- Event
}

// NewEngine creates a matching engine for the given market.
func NewEngine(marketID string) *Engine {
	return &Engine{
		marketID:   marketID,
		orderbooks: make(map[string]*Orderbook),
	}
}

// PlaceOrder validates an order, attempts to match it against the opposite
// side of the orderbook, and places any remaining quantity on the book.
func (e *Engine) PlaceOrder(order *domain.Order) (*MatchResult, error) {
	// Validate price.
	if order.Price.LessThan(minPrice) || order.Price.GreaterThan(maxPrice) {
		return nil, errInvalidPrice
	}

	// Validate quantity.
	if order.Quantity.LessThanOrEqual(decimal.Zero) {
		return nil, errInvalidQuantity
	}

	ob := e.getOrCreateOrderbook(order.OutcomeID)

	// Hold the orderbook lock for both matching and resting phases to ensure
	// atomicity.
	ob.mu.Lock()
	trades := e.matchOrder(order, ob)

	result := &MatchResult{
		Trades: trades,
	}

	// Check if there's remaining quantity to rest on the book.
	remainingQty := order.Quantity.Sub(order.FilledQty)
	if remainingQty.GreaterThan(decimal.Zero) {
		// Update order status.
		if order.FilledQty.GreaterThan(decimal.Zero) {
			order.Status = domain.OrderStatusPartial
		}
		order.UpdatedAt = time.Now()

		ob.addOrderUnlocked(order)
		result.Resting = order
	} else {
		order.Status = domain.OrderStatusFilled
		order.UpdatedAt = time.Now()
	}
	ob.mu.Unlock()

	// Emit trade events (outside the lock).
	for _, trade := range trades {
		e.emit(Event{
			Type:      EventTrade,
			MarketID:  e.marketID,
			OutcomeID: order.OutcomeID,
			Trade:     trade,
		})
	}

	// Emit book update if the order rested.
	if result.Resting != nil {
		e.emit(Event{
			Type:      EventBookUpdate,
			MarketID:  e.marketID,
			OutcomeID: order.OutcomeID,
			Orderbook: ob,
		})
	}

	return result, nil
}

// matchOrder attempts to match the incoming order against the opposite side.
// Must be called with ob.mu held.
func (e *Engine) matchOrder(taker *domain.Order, ob *Orderbook) []*domain.Trade {
	var trades []*domain.Trade
	var skippedOrders []*domain.Order

	for {
		remainingQty := taker.Quantity.Sub(taker.FilledQty)
		if remainingQty.LessThanOrEqual(decimal.Zero) {
			break
		}

		// Get the best opposing order.
		var maker *domain.Order
		if taker.Side == domain.OrderSideBuy {
			maker = ob.bestAskUnlocked()
		} else {
			maker = ob.bestBidUnlocked()
		}

		if maker == nil {
			break
		}

		// Check if prices cross.
		if taker.Side == domain.OrderSideBuy {
			// Buy order: taker price must be >= maker (ask) price.
			if taker.Price.LessThan(maker.Price) {
				break
			}
		} else {
			// Sell order: taker price must be <= maker (bid) price.
			if taker.Price.GreaterThan(maker.Price) {
				break
			}
		}

		// Self-trade prevention: skip orders from the same user.
		if taker.UserID == maker.UserID {
			// Temporarily dequeue the maker so the next iteration sees the
			// next best order. We will re-add all skipped orders after
			// matching completes.
			if taker.Side == domain.OrderSideBuy {
				ob.dequeueBestAskUnlocked()
			} else {
				ob.dequeueBestBidUnlocked()
			}
			skippedOrders = append(skippedOrders, maker)
			continue
		}

		// Determine trade quantity.
		makerRemainingQty := maker.Quantity.Sub(maker.FilledQty)
		tradeQty := decimal.Min(remainingQty, makerRemainingQty)

		// Execute the trade at the maker (resting) price.
		trade := &domain.Trade{
			ID:           uuid.New().String(),
			MarketID:     taker.MarketID,
			OutcomeID:    taker.OutcomeID,
			MakerOrderID: maker.ID,
			TakerOrderID: taker.ID,
			MakerUserID:  maker.UserID,
			TakerUserID:  taker.UserID,
			Price:        maker.Price,
			Quantity:     tradeQty,
			CreatedAt:    time.Now(),
		}

		// Update filled quantities.
		taker.FilledQty = taker.FilledQty.Add(tradeQty)
		maker.FilledQty = maker.FilledQty.Add(tradeQty)

		// Update maker status and remove from book if fully filled.
		makerNewRemaining := maker.Quantity.Sub(maker.FilledQty)
		if makerNewRemaining.LessThanOrEqual(decimal.Zero) {
			maker.Status = domain.OrderStatusFilled
			maker.UpdatedAt = time.Now()
			// Remove from the book.
			if taker.Side == domain.OrderSideBuy {
				ob.dequeueBestAskUnlocked()
			} else {
				ob.dequeueBestBidUnlocked()
			}
		} else {
			maker.Status = domain.OrderStatusPartial
			maker.UpdatedAt = time.Now()
		}

		trades = append(trades, trade)
	}

	// Re-add any self-trade orders that were temporarily dequeued.
	for _, skipped := range skippedOrders {
		ob.addOrderUnlocked(skipped)
	}

	return trades
}

// CancelOrder cancels an order, removing it from the book.
func (e *Engine) CancelOrder(outcomeID, orderID string) (*domain.Order, error) {
	e.mu.RLock()
	ob, exists := e.orderbooks[outcomeID]
	e.mu.RUnlock()

	if !exists {
		return nil, errOrderNotFound
	}

	order, found := ob.CancelOrder(orderID)
	if !found {
		return nil, errOrderNotFound
	}

	order.Status = domain.OrderStatusCancelled
	order.UpdatedAt = time.Now()

	return order, nil
}

// GetOrderbook returns the orderbook for the given outcome, or nil if none exists.
func (e *Engine) GetOrderbook(outcomeID string) *Orderbook {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.orderbooks[outcomeID]
}

// Subscribe registers a channel to receive events from the engine.
func (e *Engine) Subscribe(ch chan<- Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subscribers = append(e.subscribers, ch)
}

// getOrCreateOrderbook returns the orderbook for the given outcome, creating
// one lazily if it does not yet exist.
func (e *Engine) getOrCreateOrderbook(outcomeID string) *Orderbook {
	e.mu.RLock()
	ob, exists := e.orderbooks[outcomeID]
	e.mu.RUnlock()

	if exists {
		return ob
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock.
	ob, exists = e.orderbooks[outcomeID]
	if exists {
		return ob
	}

	ob = NewOrderbook(outcomeID)
	e.orderbooks[outcomeID] = ob
	return ob
}

// emit sends an event to all subscribers. Non-blocking: if a subscriber's
// channel is full, the event is dropped for that subscriber.
func (e *Engine) emit(event Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, ch := range e.subscribers {
		select {
		case ch <- event:
		default:
			// Drop the event if the channel is full.
		}
	}
}

// bestBidUnlocked returns the best bid without locking. Must be called with ob.mu held.
func (ob *Orderbook) bestBidUnlocked() *domain.Order {
	if len(ob.bids) == 0 {
		return nil
	}
	return ob.bids[0].queue.Peek()
}

// bestAskUnlocked returns the best ask without locking. Must be called with ob.mu held.
func (ob *Orderbook) bestAskUnlocked() *domain.Order {
	if len(ob.asks) == 0 {
		return nil
	}
	return ob.asks[0].queue.Peek()
}

// dequeueBestBidUnlocked removes the front order from the best bid level.
// Must be called with ob.mu held.
func (ob *Orderbook) dequeueBestBidUnlocked() {
	if len(ob.bids) == 0 {
		return
	}
	pq := ob.bids[0]
	front := pq.queue.Dequeue()
	if front != nil {
		delete(ob.orderSide, front.ID)
		delete(ob.orderMap, front.ID)
	}
	if pq.queue.Len() == 0 {
		ob.bids = ob.bids[1:]
	}
}

// dequeueBestAskUnlocked removes the front order from the best ask level.
// Must be called with ob.mu held.
func (ob *Orderbook) dequeueBestAskUnlocked() {
	if len(ob.asks) == 0 {
		return
	}
	pq := ob.asks[0]
	front := pq.queue.Dequeue()
	if front != nil {
		delete(ob.orderSide, front.ID)
		delete(ob.orderMap, front.ID)
	}
	if pq.queue.Len() == 0 {
		ob.asks = ob.asks[1:]
	}
}
