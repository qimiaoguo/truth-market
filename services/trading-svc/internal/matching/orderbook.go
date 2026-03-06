package matching

import (
	"sort"
	"sync"

	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// PriceLevel represents aggregated order information at a single price point.
type PriceLevel struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
	Count    int
}

// priceQueue pairs a price with its FIFO order queue.
type priceQueue struct {
	price decimal.Decimal
	queue *OrderQueue
}

// Orderbook manages buy (bid) and sell (ask) orders for a single outcome.
// Bids are sorted highest price first; asks are sorted lowest price first.
// Within the same price level, orders follow FIFO ordering.
type Orderbook struct {
	mu        sync.RWMutex
	outcomeID string
	bids      []*priceQueue                // sorted by price descending
	asks      []*priceQueue                // sorted by price ascending
	orderSide map[string]domain.OrderSide  // orderID -> side for fast cancel lookup
	orderMap  map[string]*domain.Order     // orderID -> order pointer for returning on cancel
}

// NewOrderbook creates an empty orderbook for the given outcome.
func NewOrderbook(outcomeID string) *Orderbook {
	return &Orderbook{
		outcomeID: outcomeID,
		bids:      make([]*priceQueue, 0),
		asks:      make([]*priceQueue, 0),
		orderSide: make(map[string]domain.OrderSide),
		orderMap:  make(map[string]*domain.Order),
	}
}

// AddOrder inserts an order into the appropriate side of the book.
func (ob *Orderbook) AddOrder(order *domain.Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.addOrderUnlocked(order)
}

// addOrderUnlocked inserts an order without locking. Must be called with ob.mu held.
func (ob *Orderbook) addOrderUnlocked(order *domain.Order) {
	ob.orderSide[order.ID] = order.Side
	ob.orderMap[order.ID] = order

	if order.Side == domain.OrderSideBuy {
		ob.addToBids(order)
	} else {
		ob.addToAsks(order)
	}
}

// addToBids inserts into the bid side, maintaining descending price order.
func (ob *Orderbook) addToBids(order *domain.Order) {
	// Binary search for the insertion point (bids sorted descending).
	idx := sort.Search(len(ob.bids), func(i int) bool {
		return ob.bids[i].price.LessThan(order.Price)
	})

	// Check if there's an existing level at this price.
	if idx > 0 && ob.bids[idx-1].price.Equal(order.Price) {
		ob.bids[idx-1].queue.Enqueue(order)
		return
	}

	// Insert a new price level at idx.
	pq := &priceQueue{
		price: order.Price,
		queue: NewOrderQueue(),
	}
	pq.queue.Enqueue(order)

	ob.bids = append(ob.bids, nil)
	copy(ob.bids[idx+1:], ob.bids[idx:])
	ob.bids[idx] = pq
}

// addToAsks inserts into the ask side, maintaining ascending price order.
func (ob *Orderbook) addToAsks(order *domain.Order) {
	// Binary search for the insertion point (asks sorted ascending).
	idx := sort.Search(len(ob.asks), func(i int) bool {
		return ob.asks[i].price.GreaterThan(order.Price)
	})

	// Check if there's an existing level at this price.
	if idx > 0 && ob.asks[idx-1].price.Equal(order.Price) {
		ob.asks[idx-1].queue.Enqueue(order)
		return
	}

	// Insert a new price level at idx.
	pq := &priceQueue{
		price: order.Price,
		queue: NewOrderQueue(),
	}
	pq.queue.Enqueue(order)

	ob.asks = append(ob.asks, nil)
	copy(ob.asks[idx+1:], ob.asks[idx:])
	ob.asks[idx] = pq
}

// CancelOrder removes an order by ID from the book.
// Returns the order and true if found, or nil and false otherwise.
func (ob *Orderbook) CancelOrder(orderID string) (*domain.Order, bool) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	side, ok := ob.orderSide[orderID]
	if !ok {
		return nil, false
	}

	order := ob.orderMap[orderID]

	var levels *[]*priceQueue
	if side == domain.OrderSideBuy {
		levels = &ob.bids
	} else {
		levels = &ob.asks
	}

	for i, pq := range *levels {
		if pq.queue.Remove(orderID) {
			delete(ob.orderSide, orderID)
			delete(ob.orderMap, orderID)

			// Remove empty price level.
			if pq.queue.Len() == 0 {
				*levels = append((*levels)[:i], (*levels)[i+1:]...)
			}
			return order, true
		}
	}

	// Should not reach here if orderSide map is consistent.
	delete(ob.orderSide, orderID)
	delete(ob.orderMap, orderID)
	return nil, false
}

// BestBid returns the highest-priced buy order, or nil if no bids exist.
func (ob *Orderbook) BestBid() *domain.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if len(ob.bids) == 0 {
		return nil
	}
	return ob.bids[0].queue.Peek()
}

// BestAsk returns the lowest-priced sell order, or nil if no asks exist.
func (ob *Orderbook) BestAsk() *domain.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if len(ob.asks) == 0 {
		return nil
	}
	return ob.asks[0].queue.Peek()
}

// GetBids returns all bid orders sorted by price descending, then FIFO within each level.
func (ob *Orderbook) GetBids() []*domain.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	var result []*domain.Order
	for _, pq := range ob.bids {
		result = append(result, pq.queue.Orders()...)
	}
	return result
}

// GetAsks returns all ask orders sorted by price ascending, then FIFO within each level.
func (ob *Orderbook) GetAsks() []*domain.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	var result []*domain.Order
	for _, pq := range ob.asks {
		result = append(result, pq.queue.Orders()...)
	}
	return result
}

// GetDepth returns aggregated price levels for both sides, limited to the given number of levels.
// Bid levels are returned highest price first; ask levels are returned lowest price first.
func (ob *Orderbook) GetDepth(levels int) (bidLevels, askLevels []PriceLevel) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bidLevels = ob.aggregateLevels(ob.bids, levels)
	askLevels = ob.aggregateLevels(ob.asks, levels)
	return bidLevels, askLevels
}

func (ob *Orderbook) aggregateLevels(pqs []*priceQueue, maxLevels int) []PriceLevel {
	n := len(pqs)
	if n > maxLevels {
		n = maxLevels
	}
	result := make([]PriceLevel, 0, n)
	for i := 0; i < n; i++ {
		pq := pqs[i]
		totalQty := decimal.Zero
		orders := pq.queue.Orders()
		for _, o := range orders {
			remainingQty := o.Quantity.Sub(o.FilledQty)
			totalQty = totalQty.Add(remainingQty)
		}
		result = append(result, PriceLevel{
			Price:    pq.price,
			Quantity: totalQty,
			Count:    pq.queue.Len(),
		})
	}
	return result
}

