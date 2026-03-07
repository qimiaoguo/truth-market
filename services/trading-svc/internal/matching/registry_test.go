package matching

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/truthmarket/truth-market/pkg/domain"
)

func TestRestoreOrder_AppearsInOrderbookDepth(t *testing.T) {
	reg := NewRegistry()

	order := &domain.Order{
		ID:        "order-1",
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "outcome-1",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.60),
		Quantity:  decimal.NewFromFloat(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	reg.RestoreOrder(order)

	bids, asks := reg.GetOrderbookDepth("outcome-1", 10)

	assert.Len(t, bids, 1)
	assert.Empty(t, asks)
	assert.True(t, bids[0].Price.Equal(decimal.NewFromFloat(0.60)))
	assert.True(t, bids[0].Quantity.Equal(decimal.NewFromFloat(10)))
}

func TestRestoreOrder_MultipleMarketsAndOutcomes(t *testing.T) {
	reg := NewRegistry()

	orders := []*domain.Order{
		{
			ID: "o1", UserID: "u1", MarketID: "m1", OutcomeID: "oc1",
			Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.50),
			Quantity: decimal.NewFromFloat(5), FilledQty: decimal.Zero,
			Status: domain.OrderStatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			ID: "o2", UserID: "u2", MarketID: "m1", OutcomeID: "oc1",
			Side: domain.OrderSideSell, Price: decimal.NewFromFloat(0.70),
			Quantity: decimal.NewFromFloat(3), FilledQty: decimal.Zero,
			Status: domain.OrderStatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			ID: "o3", UserID: "u1", MarketID: "m2", OutcomeID: "oc2",
			Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.30),
			Quantity: decimal.NewFromFloat(8), FilledQty: decimal.Zero,
			Status: domain.OrderStatusOpen, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}

	for _, o := range orders {
		reg.RestoreOrder(o)
	}

	// market-1, outcome-1: 1 bid, 1 ask
	bids1, asks1 := reg.GetOrderbookDepth("oc1", 10)
	assert.Len(t, bids1, 1)
	assert.Len(t, asks1, 1)
	assert.True(t, bids1[0].Price.Equal(decimal.NewFromFloat(0.50)))
	assert.True(t, asks1[0].Price.Equal(decimal.NewFromFloat(0.70)))

	// market-2, outcome-2: 1 bid, 0 asks
	bids2, asks2 := reg.GetOrderbookDepth("oc2", 10)
	assert.Len(t, bids2, 1)
	assert.Empty(t, asks2)
	assert.True(t, bids2[0].Price.Equal(decimal.NewFromFloat(0.30)))
}

func TestRestoreOrder_SamePriceLevelAggregates(t *testing.T) {
	reg := NewRegistry()

	for i, id := range []string{"a1", "a2", "a3"} {
		reg.RestoreOrder(&domain.Order{
			ID: id, UserID: "u1", MarketID: "m1", OutcomeID: "oc1",
			Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.55),
			Quantity: decimal.NewFromFloat(10), FilledQty: decimal.Zero,
			Status:    domain.OrderStatusOpen,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
			UpdatedAt: time.Now(),
		})
	}

	bids, _ := reg.GetOrderbookDepth("oc1", 10)
	assert.Len(t, bids, 1)
	assert.True(t, bids[0].Quantity.Equal(decimal.NewFromFloat(30)))
	assert.Equal(t, 3, bids[0].Count)
}
