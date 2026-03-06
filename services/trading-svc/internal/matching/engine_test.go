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

// newEngineOrder creates an order for engine tests with full control over
// all relevant fields. The ID format is deterministic for easy assertion.
func newEngineOrder(id, userID, outcomeID string, side domain.OrderSide, price float64, qty float64) *domain.Order {
	return &domain.Order{
		ID:        id,
		UserID:    userID,
		MarketID:  "market-1",
		OutcomeID: outcomeID,
		Side:      side,
		Price:     decimal.NewFromFloat(price),
		Quantity:  decimal.NewFromFloat(qty),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// collectEvents drains up to n events from ch with a short timeout.
// Returns whatever events were received before the timeout.
func collectEvents(ch <-chan Event, n int) []Event {
	var events []Event
	timeout := time.After(100 * time.Millisecond)
	for i := 0; i < n; i++ {
		select {
		case ev := <-ch:
			events = append(events, ev)
		case <-timeout:
			return events
		}
	}
	return events
}

// ---------------------------------------------------------------------------
// Tests: Basic matching
// ---------------------------------------------------------------------------

func TestEngine_BuyMatchesSell_AtSamePrice(t *testing.T) {
	engine := NewEngine("market-1")

	// Place a sell order first (it will rest on the book).
	sell := newEngineOrder("sell-1", "user-seller", "outcome-1", domain.OrderSideSell, 0.50, 10)
	result, err := engine.PlaceOrder(sell)
	require.NoError(t, err)
	assert.Empty(t, result.Trades, "sell order with empty book should produce no trades")
	assert.NotNil(t, result.Resting, "sell order should rest on the book")

	// Place a buy order at the same price -- should match.
	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.50, 10)
	result, err = engine.PlaceOrder(buy)
	require.NoError(t, err)

	require.Len(t, result.Trades, 1, "should produce exactly 1 trade")
	trade := result.Trades[0]

	assert.Equal(t, "sell-1", trade.MakerOrderID, "maker should be the resting sell order")
	assert.Equal(t, "buy-1", trade.TakerOrderID, "taker should be the incoming buy order")
	assert.Equal(t, "user-seller", trade.MakerUserID)
	assert.Equal(t, "user-buyer", trade.TakerUserID)
	assert.True(t, trade.Price.Equal(decimal.NewFromFloat(0.50)),
		"trade price should be 0.50 (maker price), got %s", trade.Price)
	assert.True(t, trade.Quantity.Equal(decimal.NewFromFloat(10)),
		"trade quantity should be 10, got %s", trade.Quantity)
	assert.Equal(t, "market-1", trade.MarketID)
	assert.Equal(t, "outcome-1", trade.OutcomeID)
	assert.NotEmpty(t, trade.ID, "trade should have an assigned ID")

	// Both orders fully filled -- nothing should rest.
	assert.Nil(t, result.Resting, "both orders fully filled, nothing should rest")

	// Book should be empty.
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	assert.Nil(t, ob.BestBid(), "book should have no bids")
	assert.Nil(t, ob.BestAsk(), "book should have no asks")
}

func TestEngine_BuyMatchesSell_BuyHigherThanAsk(t *testing.T) {
	engine := NewEngine("market-1")

	// Sell resting at 0.50.
	sell := newEngineOrder("sell-1", "user-seller", "outcome-1", domain.OrderSideSell, 0.50, 10)
	_, err := engine.PlaceOrder(sell)
	require.NoError(t, err)

	// Buy at 0.60 -- price crosses the spread, should match at maker (sell) price of 0.50.
	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.60, 10)
	result, err := engine.PlaceOrder(buy)
	require.NoError(t, err)

	require.Len(t, result.Trades, 1, "should produce a trade when buy price > ask")
	trade := result.Trades[0]

	// Trade executes at the MAKER price (0.50), not the taker price (0.60).
	assert.True(t, trade.Price.Equal(decimal.NewFromFloat(0.50)),
		"trade should execute at maker price 0.50, got %s", trade.Price)
	assert.True(t, trade.Quantity.Equal(decimal.NewFromFloat(10)),
		"trade quantity should be 10, got %s", trade.Quantity)
	assert.Nil(t, result.Resting, "both orders fully filled")
}

func TestEngine_NoMatch_BuyBelowAsk(t *testing.T) {
	engine := NewEngine("market-1")

	// Sell at 0.50.
	sell := newEngineOrder("sell-1", "user-seller", "outcome-1", domain.OrderSideSell, 0.50, 10)
	_, err := engine.PlaceOrder(sell)
	require.NoError(t, err)

	// Buy at 0.40 -- below the ask, should NOT match.
	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.40, 10)
	result, err := engine.PlaceOrder(buy)
	require.NoError(t, err)

	assert.Empty(t, result.Trades, "no trade should occur when buy price < ask price")
	assert.NotNil(t, result.Resting, "buy order should rest on the book")

	// Verify both orders are on the book.
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	assert.NotNil(t, ob.BestBid(), "book should have a bid")
	assert.NotNil(t, ob.BestAsk(), "book should have an ask")
	assert.True(t, ob.BestBid().Price.Equal(decimal.NewFromFloat(0.40)),
		"best bid should be 0.40")
	assert.True(t, ob.BestAsk().Price.Equal(decimal.NewFromFloat(0.50)),
		"best ask should be 0.50")
}

func TestEngine_PartialFill(t *testing.T) {
	engine := NewEngine("market-1")

	// Sell 5 @ 0.50 (rests on book).
	sell := newEngineOrder("sell-1", "user-seller", "outcome-1", domain.OrderSideSell, 0.50, 5)
	_, err := engine.PlaceOrder(sell)
	require.NoError(t, err)

	// Buy 10 @ 0.50 -- only 5 available to match, 5 should remain.
	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.50, 10)
	result, err := engine.PlaceOrder(buy)
	require.NoError(t, err)

	require.Len(t, result.Trades, 1, "should produce 1 trade for partial fill")
	trade := result.Trades[0]

	assert.True(t, trade.Quantity.Equal(decimal.NewFromFloat(5)),
		"trade should be for 5 shares (the available sell quantity), got %s", trade.Quantity)

	// Buy order should partially rest with remaining quantity 5.
	require.NotNil(t, result.Resting, "partially filled buy order should rest on the book")
	remainingQty := result.Resting.Quantity.Sub(result.Resting.FilledQty)
	assert.True(t, remainingQty.Equal(decimal.NewFromFloat(5)),
		"remaining quantity should be 5, got %s", remainingQty)

	// The sell side should be empty, bid side should have the resting buy.
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	assert.Nil(t, ob.BestAsk(), "sell side should be empty after full fill of sell")
	assert.NotNil(t, ob.BestBid(), "partially filled buy should be on the bid side")
}

// ---------------------------------------------------------------------------
// Tests: Price/Time priority
// ---------------------------------------------------------------------------

func TestEngine_PriceTimePriority_BestPriceFirst(t *testing.T) {
	engine := NewEngine("market-1")

	// Two sells at different prices.
	sell1 := newEngineOrder("sell-expensive", "user-s1", "outcome-1", domain.OrderSideSell, 0.50, 10)
	sell2 := newEngineOrder("sell-cheap", "user-s2", "outcome-1", domain.OrderSideSell, 0.40, 10)

	_, err := engine.PlaceOrder(sell1)
	require.NoError(t, err)
	_, err = engine.PlaceOrder(sell2)
	require.NoError(t, err)

	// Buy at 0.55 -- should match the cheaper sell first (0.40).
	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.55, 10)
	result, err := engine.PlaceOrder(buy)
	require.NoError(t, err)

	require.Len(t, result.Trades, 1, "should match with the best-priced sell")
	trade := result.Trades[0]

	assert.Equal(t, "sell-cheap", trade.MakerOrderID,
		"should match with the cheaper sell (0.40) first")
	assert.True(t, trade.Price.Equal(decimal.NewFromFloat(0.40)),
		"trade price should be maker price 0.40, got %s", trade.Price)

	// The more expensive sell should still be on the book.
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	bestAsk := ob.BestAsk()
	require.NotNil(t, bestAsk, "more expensive sell should still be resting")
	assert.Equal(t, "sell-expensive", bestAsk.ID)
}

func TestEngine_PriceTimePriority_SamePriceFIFO(t *testing.T) {
	engine := NewEngine("market-1")

	// Two sells at the same price. First one placed should be matched first.
	sell1 := newEngineOrder("sell-first", "user-s1", "outcome-1", domain.OrderSideSell, 0.50, 10)
	sell1.CreatedAt = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	sell2 := newEngineOrder("sell-second", "user-s2", "outcome-1", domain.OrderSideSell, 0.50, 10)
	sell2.CreatedAt = time.Date(2026, 1, 1, 12, 0, 1, 0, time.UTC)

	_, err := engine.PlaceOrder(sell1)
	require.NoError(t, err)
	_, err = engine.PlaceOrder(sell2)
	require.NoError(t, err)

	// Buy that matches only 10 of the 20 available.
	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.50, 10)
	result, err := engine.PlaceOrder(buy)
	require.NoError(t, err)

	require.Len(t, result.Trades, 1, "should match with one sell")
	assert.Equal(t, "sell-first", result.Trades[0].MakerOrderID,
		"FIFO: first placed sell at same price should be matched first")

	// sell-second should still be resting.
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	bestAsk := ob.BestAsk()
	require.NotNil(t, bestAsk)
	assert.Equal(t, "sell-second", bestAsk.ID, "second sell should remain on book")
}

// ---------------------------------------------------------------------------
// Tests: Multiple fills
// ---------------------------------------------------------------------------

func TestEngine_MultipleFills_SingleTaker(t *testing.T) {
	engine := NewEngine("market-1")

	// Place three sell orders at ascending prices.
	sell1 := newEngineOrder("sell-1", "user-s1", "outcome-1", domain.OrderSideSell, 0.40, 5)
	sell2 := newEngineOrder("sell-2", "user-s2", "outcome-1", domain.OrderSideSell, 0.45, 8)
	sell3 := newEngineOrder("sell-3", "user-s3", "outcome-1", domain.OrderSideSell, 0.50, 10)

	_, err := engine.PlaceOrder(sell1)
	require.NoError(t, err)
	_, err = engine.PlaceOrder(sell2)
	require.NoError(t, err)
	_, err = engine.PlaceOrder(sell3)
	require.NoError(t, err)

	// Buy 20 @ 0.50 -- should sweep through sell-1 (5), sell-2 (8), sell-3 (7).
	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.50, 20)
	result, err := engine.PlaceOrder(buy)
	require.NoError(t, err)

	require.Len(t, result.Trades, 3, "should produce 3 trades sweeping the book")

	// Trade 1: matches sell-1 (cheapest) for 5 @ 0.40.
	assert.Equal(t, "sell-1", result.Trades[0].MakerOrderID)
	assert.True(t, result.Trades[0].Quantity.Equal(decimal.NewFromFloat(5)),
		"first trade qty should be 5, got %s", result.Trades[0].Quantity)
	assert.True(t, result.Trades[0].Price.Equal(decimal.NewFromFloat(0.40)),
		"first trade price should be 0.40, got %s", result.Trades[0].Price)

	// Trade 2: matches sell-2 for 8 @ 0.45.
	assert.Equal(t, "sell-2", result.Trades[1].MakerOrderID)
	assert.True(t, result.Trades[1].Quantity.Equal(decimal.NewFromFloat(8)),
		"second trade qty should be 8, got %s", result.Trades[1].Quantity)
	assert.True(t, result.Trades[1].Price.Equal(decimal.NewFromFloat(0.45)),
		"second trade price should be 0.45, got %s", result.Trades[1].Price)

	// Trade 3: matches sell-3 for remaining 7 @ 0.50.
	assert.Equal(t, "sell-3", result.Trades[2].MakerOrderID)
	assert.True(t, result.Trades[2].Quantity.Equal(decimal.NewFromFloat(7)),
		"third trade qty should be 7, got %s", result.Trades[2].Quantity)
	assert.True(t, result.Trades[2].Price.Equal(decimal.NewFromFloat(0.50)),
		"third trade price should be 0.50, got %s", result.Trades[2].Price)

	// Total filled: 5 + 8 + 7 = 20. Buy order fully filled, nothing rests.
	assert.Nil(t, result.Resting, "buy order should be fully filled, nothing rests")

	// sell-3 should have 3 remaining on the book (10 - 7 = 3).
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	bestAsk := ob.BestAsk()
	require.NotNil(t, bestAsk, "sell-3 should have a partial remainder on the book")
	assert.Equal(t, "sell-3", bestAsk.ID)
	remainingQty := bestAsk.Quantity.Sub(bestAsk.FilledQty)
	assert.True(t, remainingQty.Equal(decimal.NewFromFloat(3)),
		"sell-3 remainder should be 3, got %s", remainingQty)
}

func TestEngine_SellMatchesBuy(t *testing.T) {
	engine := NewEngine("market-1")

	// Place buy orders first (they rest on the book).
	buy1 := newEngineOrder("buy-1", "user-b1", "outcome-1", domain.OrderSideBuy, 0.55, 10)
	buy2 := newEngineOrder("buy-2", "user-b2", "outcome-1", domain.OrderSideBuy, 0.50, 10)

	_, err := engine.PlaceOrder(buy1)
	require.NoError(t, err)
	_, err = engine.PlaceOrder(buy2)
	require.NoError(t, err)

	// Incoming sell at 0.50 -- should match buy-1 first (highest bid at 0.55).
	sell := newEngineOrder("sell-1", "user-seller", "outcome-1", domain.OrderSideSell, 0.50, 10)
	result, err := engine.PlaceOrder(sell)
	require.NoError(t, err)

	require.Len(t, result.Trades, 1, "sell should match with the best bid")
	trade := result.Trades[0]

	assert.Equal(t, "buy-1", trade.MakerOrderID,
		"sell should match the highest bid (buy-1 at 0.55)")
	assert.Equal(t, "sell-1", trade.TakerOrderID)
	assert.True(t, trade.Price.Equal(decimal.NewFromFloat(0.55)),
		"trade price should be maker price 0.55, got %s", trade.Price)
	assert.Nil(t, result.Resting, "sell fully filled")

	// buy-2 should still be resting on the book.
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	bestBid := ob.BestBid()
	require.NotNil(t, bestBid, "buy-2 should remain on the book")
	assert.Equal(t, "buy-2", bestBid.ID)
}

// ---------------------------------------------------------------------------
// Tests: Edge cases
// ---------------------------------------------------------------------------

func TestEngine_SelfTrade_Prevention(t *testing.T) {
	engine := NewEngine("market-1")

	// User places a sell order.
	sell := newEngineOrder("sell-1", "user-same", "outcome-1", domain.OrderSideSell, 0.50, 10)
	_, err := engine.PlaceOrder(sell)
	require.NoError(t, err)

	// Same user places a buy order at a crossing price.
	buy := newEngineOrder("buy-1", "user-same", "outcome-1", domain.OrderSideBuy, 0.50, 10)
	result, err := engine.PlaceOrder(buy)
	require.NoError(t, err)

	// Self-trade should be prevented -- no trades produced.
	assert.Empty(t, result.Trades, "self-trade should be prevented: same user on both sides")

	// Buy order should rest on the book (skipped the matching sell).
	assert.NotNil(t, result.Resting, "buy order should rest on book after skipping self-match")

	// Both orders should be on the book.
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	assert.NotNil(t, ob.BestBid(), "buy should be resting on the bid side")
	assert.NotNil(t, ob.BestAsk(), "sell should still be resting on the ask side")
}

func TestEngine_ZeroQuantity_Rejected(t *testing.T) {
	engine := NewEngine("market-1")

	// Zero quantity.
	order := newEngineOrder("bad-1", "user-1", "outcome-1", domain.OrderSideBuy, 0.50, 0)
	result, err := engine.PlaceOrder(order)
	assert.Error(t, err, "zero quantity should be rejected")
	assert.Nil(t, result, "no result should be returned for invalid order")

	// Negative quantity.
	order2 := newEngineOrder("bad-2", "user-1", "outcome-1", domain.OrderSideBuy, 0.50, -5)
	result2, err2 := engine.PlaceOrder(order2)
	assert.Error(t, err2, "negative quantity should be rejected")
	assert.Nil(t, result2, "no result should be returned for invalid order")
}

func TestEngine_InvalidPrice_Rejected(t *testing.T) {
	engine := NewEngine("market-1")

	// Price below minimum (< 0.01).
	order := newEngineOrder("bad-price-low", "user-1", "outcome-1", domain.OrderSideBuy, 0.001, 10)
	result, err := engine.PlaceOrder(order)
	assert.Error(t, err, "price below 0.01 should be rejected")
	assert.Nil(t, result, "no result for invalid price")

	// Price at exactly zero.
	order2 := newEngineOrder("bad-price-zero", "user-1", "outcome-1", domain.OrderSideBuy, 0, 10)
	result2, err2 := engine.PlaceOrder(order2)
	assert.Error(t, err2, "price of 0 should be rejected")
	assert.Nil(t, result2, "no result for invalid price")

	// Price above maximum (> 0.99).
	order3 := newEngineOrder("bad-price-high", "user-1", "outcome-1", domain.OrderSideBuy, 1.0, 10)
	result3, err3 := engine.PlaceOrder(order3)
	assert.Error(t, err3, "price of 1.0 should be rejected (max is 0.99)")
	assert.Nil(t, result3, "no result for invalid price")

	// Negative price.
	order4 := newEngineOrder("bad-price-neg", "user-1", "outcome-1", domain.OrderSideBuy, -0.50, 10)
	result4, err4 := engine.PlaceOrder(order4)
	assert.Error(t, err4, "negative price should be rejected")
	assert.Nil(t, result4, "no result for invalid price")

	// Valid boundary prices should be accepted.
	orderLow := newEngineOrder("ok-low", "user-1", "outcome-1", domain.OrderSideBuy, 0.01, 10)
	resultLow, errLow := engine.PlaceOrder(orderLow)
	assert.NoError(t, errLow, "price of 0.01 should be valid")
	assert.NotNil(t, resultLow, "valid order should return a result")

	orderHigh := newEngineOrder("ok-high", "user-1", "outcome-1", domain.OrderSideSell, 0.99, 10)
	resultHigh, errHigh := engine.PlaceOrder(orderHigh)
	assert.NoError(t, errHigh, "price of 0.99 should be valid")
	assert.NotNil(t, resultHigh, "valid order should return a result")
}

// ---------------------------------------------------------------------------
// Tests: Cancel
// ---------------------------------------------------------------------------

func TestEngine_CancelOrder_RemovesFromBook(t *testing.T) {
	engine := NewEngine("market-1")

	// Place two orders on the book.
	buy1 := newEngineOrder("buy-1", "user-1", "outcome-1", domain.OrderSideBuy, 0.45, 10)
	buy2 := newEngineOrder("buy-2", "user-2", "outcome-1", domain.OrderSideBuy, 0.50, 5)

	_, err := engine.PlaceOrder(buy1)
	require.NoError(t, err)
	_, err = engine.PlaceOrder(buy2)
	require.NoError(t, err)

	// Cancel buy-2 (the best bid).
	cancelled, err := engine.CancelOrder("outcome-1", "buy-2")
	require.NoError(t, err)
	require.NotNil(t, cancelled, "cancelled order should be returned")
	assert.Equal(t, "buy-2", cancelled.ID, "returned order ID should match")
	assert.Equal(t, domain.OrderStatusCancelled, cancelled.Status,
		"cancelled order status should be 'cancelled'")

	// buy-1 should now be the best bid.
	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob)
	bestBid := ob.BestBid()
	require.NotNil(t, bestBid, "buy-1 should still be on the book")
	assert.Equal(t, "buy-1", bestBid.ID, "best bid should now be buy-1")

	// A new sell at 0.48 should NOT match cancelled buy-2.
	sell := newEngineOrder("sell-1", "user-seller", "outcome-1", domain.OrderSideSell, 0.48, 5)
	result, err := engine.PlaceOrder(sell)
	require.NoError(t, err)
	assert.Empty(t, result.Trades, "cancelled order should not be matchable")
}

func TestEngine_CancelNonexistent_ReturnsError(t *testing.T) {
	engine := NewEngine("market-1")

	// Cancel on an outcome that has no orders.
	_, err := engine.CancelOrder("outcome-1", "nonexistent-order")
	assert.Error(t, err, "cancelling a non-existent order should return an error")

	// Place an order, then cancel it, then try to cancel again.
	buy := newEngineOrder("buy-1", "user-1", "outcome-1", domain.OrderSideBuy, 0.50, 10)
	_, err = engine.PlaceOrder(buy)
	require.NoError(t, err)

	_, err = engine.CancelOrder("outcome-1", "buy-1")
	require.NoError(t, err, "first cancel should succeed")

	_, err = engine.CancelOrder("outcome-1", "buy-1")
	assert.Error(t, err, "cancelling an already-cancelled order should return an error")

	// Cancel on a wrong outcomeID.
	buy2 := newEngineOrder("buy-2", "user-1", "outcome-1", domain.OrderSideBuy, 0.50, 10)
	_, err = engine.PlaceOrder(buy2)
	require.NoError(t, err)

	_, err = engine.CancelOrder("outcome-wrong", "buy-2")
	assert.Error(t, err, "cancelling with wrong outcome ID should return an error")
}

// ---------------------------------------------------------------------------
// Tests: Orderbook state
// ---------------------------------------------------------------------------

func TestEngine_GetOrderbook_ReturnsDepth(t *testing.T) {
	engine := NewEngine("market-1")

	// Place a variety of orders.
	_, _ = engine.PlaceOrder(newEngineOrder("bid-1", "u1", "outcome-1", domain.OrderSideBuy, 0.45, 10))
	_, _ = engine.PlaceOrder(newEngineOrder("bid-2", "u2", "outcome-1", domain.OrderSideBuy, 0.45, 5))
	_, _ = engine.PlaceOrder(newEngineOrder("bid-3", "u3", "outcome-1", domain.OrderSideBuy, 0.40, 8))
	_, _ = engine.PlaceOrder(newEngineOrder("ask-1", "u4", "outcome-1", domain.OrderSideSell, 0.55, 10))
	_, _ = engine.PlaceOrder(newEngineOrder("ask-2", "u5", "outcome-1", domain.OrderSideSell, 0.60, 7))

	ob := engine.GetOrderbook("outcome-1")
	require.NotNil(t, ob, "engine should return an orderbook for a known outcome")

	bidLevels, askLevels := ob.GetDepth(10)

	// Bids: 0.45 (qty 15, count 2), 0.40 (qty 8, count 1).
	require.Len(t, bidLevels, 2, "should have 2 bid price levels")
	assert.True(t, bidLevels[0].Price.Equal(decimal.NewFromFloat(0.45)),
		"top bid level price should be 0.45")
	assert.True(t, bidLevels[0].Quantity.Equal(decimal.NewFromFloat(15)),
		"top bid level quantity should be 15")
	assert.Equal(t, 2, bidLevels[0].Count)

	// Asks: 0.55 (qty 10, count 1), 0.60 (qty 7, count 1).
	require.Len(t, askLevels, 2, "should have 2 ask price levels")
	assert.True(t, askLevels[0].Price.Equal(decimal.NewFromFloat(0.55)),
		"top ask level price should be 0.55")
	assert.True(t, askLevels[0].Quantity.Equal(decimal.NewFromFloat(10)),
		"top ask level quantity should be 10")
}

func TestEngine_MultipleOutcomes_IndependentBooks(t *testing.T) {
	engine := NewEngine("market-1")

	// Place orders on two different outcomes.
	sellA := newEngineOrder("sell-A", "user-seller", "outcome-A", domain.OrderSideSell, 0.50, 10)
	_, err := engine.PlaceOrder(sellA)
	require.NoError(t, err)

	sellB := newEngineOrder("sell-B", "user-seller2", "outcome-B", domain.OrderSideSell, 0.50, 10)
	_, err = engine.PlaceOrder(sellB)
	require.NoError(t, err)

	// Buy on outcome-A should NOT match sell on outcome-B.
	buyA := newEngineOrder("buy-A", "user-buyer", "outcome-A", domain.OrderSideBuy, 0.50, 10)
	result, err := engine.PlaceOrder(buyA)
	require.NoError(t, err)

	require.Len(t, result.Trades, 1, "buy on outcome-A should match sell on outcome-A")
	assert.Equal(t, "sell-A", result.Trades[0].MakerOrderID,
		"should match with sell from the same outcome")
	assert.Equal(t, "outcome-A", result.Trades[0].OutcomeID)

	// outcome-B sell should be untouched.
	obB := engine.GetOrderbook("outcome-B")
	require.NotNil(t, obB)
	bestAskB := obB.BestAsk()
	require.NotNil(t, bestAskB, "outcome-B sell should still be resting")
	assert.Equal(t, "sell-B", bestAskB.ID)

	// outcome-A book should be empty after the match.
	obA := engine.GetOrderbook("outcome-A")
	require.NotNil(t, obA)
	assert.Nil(t, obA.BestBid(), "outcome-A should have no bids after full match")
	assert.Nil(t, obA.BestAsk(), "outcome-A should have no asks after full match")
}

// ---------------------------------------------------------------------------
// Tests: Event emission
// ---------------------------------------------------------------------------

func TestEngine_Trade_EmitsTradeEvent(t *testing.T) {
	engine := NewEngine("market-1")

	// Subscribe to events.
	eventCh := make(chan Event, 10)
	engine.Subscribe(eventCh)

	// Place a sell then a matching buy.
	sell := newEngineOrder("sell-1", "user-seller", "outcome-1", domain.OrderSideSell, 0.50, 10)
	_, err := engine.PlaceOrder(sell)
	require.NoError(t, err)

	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.50, 10)
	_, err = engine.PlaceOrder(buy)
	require.NoError(t, err)

	// Collect events. We expect at least a trade event.
	events := collectEvents(eventCh, 10)

	var tradeEvents []Event
	for _, ev := range events {
		if ev.Type == EventTrade {
			tradeEvents = append(tradeEvents, ev)
		}
	}

	require.NotEmpty(t, tradeEvents, "should emit at least one trade event")
	tradeEvent := tradeEvents[0]

	assert.Equal(t, "market-1", tradeEvent.MarketID, "trade event should carry market ID")
	assert.Equal(t, "outcome-1", tradeEvent.OutcomeID, "trade event should carry outcome ID")
	require.NotNil(t, tradeEvent.Trade, "trade event should contain the trade")
	assert.Equal(t, "sell-1", tradeEvent.Trade.MakerOrderID)
	assert.Equal(t, "buy-1", tradeEvent.Trade.TakerOrderID)
	assert.True(t, tradeEvent.Trade.Price.Equal(decimal.NewFromFloat(0.50)))
	assert.True(t, tradeEvent.Trade.Quantity.Equal(decimal.NewFromFloat(10)))
}

func TestEngine_OrderPlaced_EmitsBookUpdate(t *testing.T) {
	engine := NewEngine("market-1")

	// Subscribe to events.
	eventCh := make(chan Event, 10)
	engine.Subscribe(eventCh)

	// Place a buy order that will NOT match (no sell orders exist).
	buy := newEngineOrder("buy-1", "user-buyer", "outcome-1", domain.OrderSideBuy, 0.50, 10)
	_, err := engine.PlaceOrder(buy)
	require.NoError(t, err)

	// Collect events. We expect a book update event (no trade events).
	events := collectEvents(eventCh, 10)

	var bookUpdateEvents []Event
	var tradeEvents []Event
	for _, ev := range events {
		switch ev.Type {
		case EventBookUpdate:
			bookUpdateEvents = append(bookUpdateEvents, ev)
		case EventTrade:
			tradeEvents = append(tradeEvents, ev)
		}
	}

	assert.Empty(t, tradeEvents, "no trade events should be emitted for an unmatched order")
	require.NotEmpty(t, bookUpdateEvents, "should emit at least one book update event")

	bookEvent := bookUpdateEvents[0]
	assert.Equal(t, "market-1", bookEvent.MarketID, "book update should carry market ID")
	assert.Equal(t, "outcome-1", bookEvent.OutcomeID, "book update should carry outcome ID")
	assert.NotNil(t, bookEvent.Orderbook, "book update should contain the orderbook snapshot")
}
