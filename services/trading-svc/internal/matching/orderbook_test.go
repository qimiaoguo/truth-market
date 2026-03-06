package matching

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newOrderbookOrder creates an order suitable for orderbook tests with
// explicit price and side. The createdAt parameter controls FIFO tiebreaking
// at the same price level -- earlier timestamps are matched first.
func newOrderbookOrder(id string, side domain.OrderSide, price float64, qty float64, createdAt time.Time) *domain.Order {
	return &domain.Order{
		ID:        id,
		UserID:    "user-" + id,
		MarketID:  "market-1",
		OutcomeID: "outcome-1",
		Side:      side,
		Price:     decimal.NewFromFloat(price),
		Quantity:  decimal.NewFromFloat(qty),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

// ---------------------------------------------------------------------------
// Tests: Orderbook
// ---------------------------------------------------------------------------

func TestOrderbook_AddBuyOrder_ShowsInBids(t *testing.T) {
	ob := NewOrderbook("outcome-1")
	now := time.Now()

	order := newOrderbookOrder("buy-1", domain.OrderSideBuy, 0.50, 10, now)
	ob.AddOrder(order)

	bids := ob.GetBids()
	require.Len(t, bids, 1, "should have exactly 1 bid")
	assert.Equal(t, "buy-1", bids[0].ID, "bid should be the order we added")
	assert.True(t, bids[0].Price.Equal(decimal.NewFromFloat(0.50)),
		"bid price should be 0.50, got %s", bids[0].Price)

	// Should NOT appear in asks.
	asks := ob.GetAsks()
	assert.Empty(t, asks, "buy order should not appear in asks")
}

func TestOrderbook_AddSellOrder_ShowsInAsks(t *testing.T) {
	ob := NewOrderbook("outcome-1")
	now := time.Now()

	order := newOrderbookOrder("sell-1", domain.OrderSideSell, 0.60, 5, now)
	ob.AddOrder(order)

	asks := ob.GetAsks()
	require.Len(t, asks, 1, "should have exactly 1 ask")
	assert.Equal(t, "sell-1", asks[0].ID, "ask should be the order we added")
	assert.True(t, asks[0].Price.Equal(decimal.NewFromFloat(0.60)),
		"ask price should be 0.60, got %s", asks[0].Price)

	// Should NOT appear in bids.
	bids := ob.GetBids()
	assert.Empty(t, bids, "sell order should not appear in bids")
}

func TestOrderbook_BidsOrderedByPriceDesc(t *testing.T) {
	ob := NewOrderbook("outcome-1")
	now := time.Now()

	// Insert bids at various prices in non-sorted order.
	ob.AddOrder(newOrderbookOrder("bid-low", domain.OrderSideBuy, 0.30, 10, now))
	ob.AddOrder(newOrderbookOrder("bid-high", domain.OrderSideBuy, 0.70, 10, now.Add(time.Second)))
	ob.AddOrder(newOrderbookOrder("bid-mid", domain.OrderSideBuy, 0.50, 10, now.Add(2*time.Second)))

	bids := ob.GetBids()
	require.Len(t, bids, 3, "should have 3 bids")

	// Bids should be ordered highest price first (best bid at index 0).
	assert.Equal(t, "bid-high", bids[0].ID, "highest-priced bid should be first")
	assert.Equal(t, "bid-mid", bids[1].ID, "mid-priced bid should be second")
	assert.Equal(t, "bid-low", bids[2].ID, "lowest-priced bid should be last")

	// Verify prices are strictly descending.
	for i := 1; i < len(bids); i++ {
		assert.True(t, bids[i-1].Price.GreaterThan(bids[i].Price),
			"bid[%d].Price (%s) should be > bid[%d].Price (%s)",
			i-1, bids[i-1].Price, i, bids[i].Price)
	}
}

func TestOrderbook_AsksOrderedByPriceAsc(t *testing.T) {
	ob := NewOrderbook("outcome-1")
	now := time.Now()

	// Insert asks at various prices in non-sorted order.
	ob.AddOrder(newOrderbookOrder("ask-high", domain.OrderSideSell, 0.80, 10, now))
	ob.AddOrder(newOrderbookOrder("ask-low", domain.OrderSideSell, 0.40, 10, now.Add(time.Second)))
	ob.AddOrder(newOrderbookOrder("ask-mid", domain.OrderSideSell, 0.60, 10, now.Add(2*time.Second)))

	asks := ob.GetAsks()
	require.Len(t, asks, 3, "should have 3 asks")

	// Asks should be ordered lowest price first (best ask at index 0).
	assert.Equal(t, "ask-low", asks[0].ID, "lowest-priced ask should be first")
	assert.Equal(t, "ask-mid", asks[1].ID, "mid-priced ask should be second")
	assert.Equal(t, "ask-high", asks[2].ID, "highest-priced ask should be last")

	// Verify prices are strictly ascending.
	for i := 1; i < len(asks); i++ {
		assert.True(t, asks[i-1].Price.LessThan(asks[i].Price),
			"ask[%d].Price (%s) should be < ask[%d].Price (%s)",
			i-1, asks[i-1].Price, i, asks[i].Price)
	}
}

func TestOrderbook_SamePriceFIFO(t *testing.T) {
	ob := NewOrderbook("outcome-1")
	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Insert three buy orders at the same price, with distinct timestamps.
	o1 := newOrderbookOrder("first", domain.OrderSideBuy, 0.50, 10, baseTime)
	o2 := newOrderbookOrder("second", domain.OrderSideBuy, 0.50, 5, baseTime.Add(1*time.Second))
	o3 := newOrderbookOrder("third", domain.OrderSideBuy, 0.50, 8, baseTime.Add(2*time.Second))

	ob.AddOrder(o1)
	ob.AddOrder(o2)
	ob.AddOrder(o3)

	bids := ob.GetBids()
	require.Len(t, bids, 3, "should have 3 bids at the same price level")

	// At the same price level, orders should appear in FIFO order (earliest first).
	assert.Equal(t, "first", bids[0].ID, "first inserted order should be matched first")
	assert.Equal(t, "second", bids[1].ID, "second inserted order should be next")
	assert.Equal(t, "third", bids[2].ID, "third inserted order should be last")
}

func TestOrderbook_CancelOrder_RemovesFromBook(t *testing.T) {
	ob := NewOrderbook("outcome-1")
	now := time.Now()

	o1 := newOrderbookOrder("buy-1", domain.OrderSideBuy, 0.50, 10, now)
	o2 := newOrderbookOrder("buy-2", domain.OrderSideBuy, 0.60, 5, now.Add(time.Second))
	o3 := newOrderbookOrder("sell-1", domain.OrderSideSell, 0.70, 8, now.Add(2*time.Second))

	ob.AddOrder(o1)
	ob.AddOrder(o2)
	ob.AddOrder(o3)

	// Cancel buy-1.
	cancelled, found := ob.CancelOrder("buy-1")
	assert.True(t, found, "cancel should find the order")
	require.NotNil(t, cancelled, "cancelled order should be returned")
	assert.Equal(t, "buy-1", cancelled.ID, "returned order should be the cancelled one")

	// buy-1 should no longer appear in bids.
	bids := ob.GetBids()
	require.Len(t, bids, 1, "should have 1 bid remaining after cancel")
	assert.Equal(t, "buy-2", bids[0].ID, "remaining bid should be buy-2")

	// Sell side should be unaffected.
	asks := ob.GetAsks()
	require.Len(t, asks, 1, "asks should be unaffected by cancelling a buy")

	// Cancel a non-existent order.
	_, found = ob.CancelOrder("nonexistent")
	assert.False(t, found, "cancel should return false for non-existent order")
}

func TestOrderbook_BestBidAsk(t *testing.T) {
	ob := NewOrderbook("outcome-1")
	now := time.Now()

	// Empty book: best bid and ask should be nil.
	assert.Nil(t, ob.BestBid(), "best bid should be nil on empty book")
	assert.Nil(t, ob.BestAsk(), "best ask should be nil on empty book")

	// Add some bids and asks.
	ob.AddOrder(newOrderbookOrder("bid-low", domain.OrderSideBuy, 0.40, 10, now))
	ob.AddOrder(newOrderbookOrder("bid-high", domain.OrderSideBuy, 0.55, 5, now.Add(time.Second)))
	ob.AddOrder(newOrderbookOrder("ask-high", domain.OrderSideSell, 0.80, 10, now.Add(2*time.Second)))
	ob.AddOrder(newOrderbookOrder("ask-low", domain.OrderSideSell, 0.60, 5, now.Add(3*time.Second)))

	// Best bid = highest price buy order.
	bestBid := ob.BestBid()
	require.NotNil(t, bestBid, "best bid should not be nil")
	assert.Equal(t, "bid-high", bestBid.ID, "best bid should be the highest-priced buy")
	assert.True(t, bestBid.Price.Equal(decimal.NewFromFloat(0.55)),
		"best bid price should be 0.55, got %s", bestBid.Price)

	// Best ask = lowest price sell order.
	bestAsk := ob.BestAsk()
	require.NotNil(t, bestAsk, "best ask should not be nil")
	assert.Equal(t, "ask-low", bestAsk.ID, "best ask should be the lowest-priced sell")
	assert.True(t, bestAsk.Price.Equal(decimal.NewFromFloat(0.60)),
		"best ask price should be 0.60, got %s", bestAsk.Price)
}

func TestOrderbook_GetDepth(t *testing.T) {
	ob := NewOrderbook("outcome-1")
	now := time.Now()

	// Build a book with multiple orders at various price levels.
	// Bids: 2 orders at 0.50 (total qty 15), 1 order at 0.40 (qty 10).
	ob.AddOrder(newOrderbookOrder("bid-1", domain.OrderSideBuy, 0.50, 10, now))
	ob.AddOrder(newOrderbookOrder("bid-2", domain.OrderSideBuy, 0.50, 5, now.Add(time.Second)))
	ob.AddOrder(newOrderbookOrder("bid-3", domain.OrderSideBuy, 0.40, 10, now.Add(2*time.Second)))

	// Asks: 1 order at 0.60 (qty 8), 2 orders at 0.70 (total qty 12).
	ob.AddOrder(newOrderbookOrder("ask-1", domain.OrderSideSell, 0.60, 8, now.Add(3*time.Second)))
	ob.AddOrder(newOrderbookOrder("ask-2", domain.OrderSideSell, 0.70, 7, now.Add(4*time.Second)))
	ob.AddOrder(newOrderbookOrder("ask-3", domain.OrderSideSell, 0.70, 5, now.Add(5*time.Second)))

	bidLevels, askLevels := ob.GetDepth(10)

	// Verify bid price levels.
	require.Len(t, bidLevels, 2, "should have 2 bid price levels")
	// Best bid first (highest price).
	assert.True(t, bidLevels[0].Price.Equal(decimal.NewFromFloat(0.50)),
		"first bid level price should be 0.50, got %s", bidLevels[0].Price)
	assert.True(t, bidLevels[0].Quantity.Equal(decimal.NewFromFloat(15)),
		"first bid level aggregate qty should be 15, got %s", bidLevels[0].Quantity)
	assert.Equal(t, 2, bidLevels[0].Count, "first bid level should have 2 orders")

	assert.True(t, bidLevels[1].Price.Equal(decimal.NewFromFloat(0.40)),
		"second bid level price should be 0.40, got %s", bidLevels[1].Price)
	assert.True(t, bidLevels[1].Quantity.Equal(decimal.NewFromFloat(10)),
		"second bid level aggregate qty should be 10, got %s", bidLevels[1].Quantity)
	assert.Equal(t, 1, bidLevels[1].Count, "second bid level should have 1 order")

	// Verify ask price levels.
	require.Len(t, askLevels, 2, "should have 2 ask price levels")
	// Best ask first (lowest price).
	assert.True(t, askLevels[0].Price.Equal(decimal.NewFromFloat(0.60)),
		"first ask level price should be 0.60, got %s", askLevels[0].Price)
	assert.True(t, askLevels[0].Quantity.Equal(decimal.NewFromFloat(8)),
		"first ask level aggregate qty should be 8, got %s", askLevels[0].Quantity)
	assert.Equal(t, 1, askLevels[0].Count, "first ask level should have 1 order")

	assert.True(t, askLevels[1].Price.Equal(decimal.NewFromFloat(0.70)),
		"second ask level price should be 0.70, got %s", askLevels[1].Price)
	assert.True(t, askLevels[1].Quantity.Equal(decimal.NewFromFloat(12)),
		"second ask level aggregate qty should be 12, got %s", askLevels[1].Quantity)
	assert.Equal(t, 2, askLevels[1].Count, "second ask level should have 2 orders")

	// Verify level-limiting: request only 1 level per side.
	bidLevels1, askLevels1 := ob.GetDepth(1)
	require.Len(t, bidLevels1, 1, "requesting depth of 1 should return 1 bid level")
	require.Len(t, askLevels1, 1, "requesting depth of 1 should return 1 ask level")
	assert.True(t, bidLevels1[0].Price.Equal(decimal.NewFromFloat(0.50)),
		"single bid level should be the best bid")
	assert.True(t, askLevels1[0].Price.Equal(decimal.NewFromFloat(0.60)),
		"single ask level should be the best ask")

	// Empty book depth.
	emptyOb := NewOrderbook("outcome-empty")
	emptyBids, emptyAsks := emptyOb.GetDepth(10)
	assert.Empty(t, emptyBids, "empty book should have no bid levels")
	assert.Empty(t, emptyAsks, "empty book should have no ask levels")
}
