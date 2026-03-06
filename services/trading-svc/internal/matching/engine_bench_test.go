package matching

import (
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// ---------------------------------------------------------------------------
// Benchmark helpers
// ---------------------------------------------------------------------------

// benchOrder creates an order for benchmarking with the given parameters.
func benchOrder(id string, userID string, side domain.OrderSide, price float64, qty float64) *domain.Order {
	return &domain.Order{
		ID:        id,
		UserID:    userID,
		MarketID:  "market-bench",
		OutcomeID: "outcome-bench",
		Side:      side,
		Price:     decimal.NewFromFloat(price),
		Quantity:  decimal.NewFromFloat(qty),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkEngine_PlaceOrder_NoMatch measures the throughput of placing orders
// that do not match (alternating buy/sell at non-crossing prices).
// This benchmarks the "add to book" path without matching overhead.
func BenchmarkEngine_PlaceOrder_NoMatch(b *testing.B) {
	engine := NewEngine("market-bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var side domain.OrderSide
		var price float64
		if i%2 == 0 {
			side = domain.OrderSideBuy
			price = 0.40
		} else {
			side = domain.OrderSideSell
			price = 0.60
		}
		order := benchOrder(
			fmt.Sprintf("order-%d", i),
			fmt.Sprintf("user-%d", i),
			side, price, 10,
		)
		_, _ = engine.PlaceOrder(order)
	}
}

// BenchmarkEngine_PlaceOrder_ImmediateMatch measures the throughput of placing
// orders that immediately match (buy and sell at the same price, alternating).
// Each pair produces one trade, testing the hot matching path.
func BenchmarkEngine_PlaceOrder_ImmediateMatch(b *testing.B) {
	engine := NewEngine("market-bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var side domain.OrderSide
		if i%2 == 0 {
			side = domain.OrderSideSell
		} else {
			side = domain.OrderSideBuy
		}
		order := benchOrder(
			fmt.Sprintf("order-%d", i),
			fmt.Sprintf("user-%d", i), // unique users to avoid self-trade prevention
			side, 0.50, 10,
		)
		_, _ = engine.PlaceOrder(order)
	}
}

// BenchmarkEngine_PlaceOrder_DeepBook measures matching performance against a
// deep orderbook with 1000 price levels on the sell side. A large buy order
// sweeps through many levels, stressing the price-walking path.
func BenchmarkEngine_PlaceOrder_DeepBook(b *testing.B) {
	// Pre-build a deep book outside the timing loop: 1000 sell orders at
	// prices from 0.01 to 0.99 (spread across the range), each with qty 1.
	// We rebuild the book for each iteration to ensure consistent depth.

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		engine := NewEngine("market-bench")

		// Build 1000 levels: prices from 0.010 to 0.990 in increments of ~0.001.
		// Use 1000 distinct sell orders.
		for j := 0; j < 1000; j++ {
			// Price range: 0.01 to 0.99, evenly distributed.
			price := 0.01 + float64(j)*0.00098
			if price > 0.99 {
				price = 0.99
			}
			sellOrder := benchOrder(
				fmt.Sprintf("sell-%d", j),
				fmt.Sprintf("user-seller-%d", j),
				domain.OrderSideSell, price, 1,
			)
			_, _ = engine.PlaceOrder(sellOrder)
		}

		b.StartTimer()

		// Place a large buy that sweeps through all 1000 levels.
		buy := benchOrder(
			fmt.Sprintf("buy-sweep-%d", i),
			"user-buyer",
			domain.OrderSideBuy, 0.99, 1000,
		)
		_, _ = engine.PlaceOrder(buy)
	}
}
