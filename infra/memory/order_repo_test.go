package memory

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
)

func TestListAllOpen_ReturnsOnlyOpenAndPartial(t *testing.T) {
	repo := NewOrderRepository()
	ctx := context.Background()
	now := time.Now()

	orders := []*domain.Order{
		{ID: "1", UserID: "u1", MarketID: "m1", OutcomeID: "o1", Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.50), Quantity: decimal.NewFromFloat(10), FilledQty: decimal.Zero, Status: domain.OrderStatusOpen, CreatedAt: now, UpdatedAt: now},
		{ID: "2", UserID: "u1", MarketID: "m1", OutcomeID: "o1", Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.60), Quantity: decimal.NewFromFloat(5), FilledQty: decimal.NewFromFloat(2), Status: domain.OrderStatusPartial, CreatedAt: now, UpdatedAt: now},
		{ID: "3", UserID: "u1", MarketID: "m1", OutcomeID: "o1", Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.70), Quantity: decimal.NewFromFloat(5), FilledQty: decimal.NewFromFloat(5), Status: domain.OrderStatusFilled, CreatedAt: now, UpdatedAt: now},
		{ID: "4", UserID: "u1", MarketID: "m1", OutcomeID: "o1", Side: domain.OrderSideSell, Price: decimal.NewFromFloat(0.80), Quantity: decimal.NewFromFloat(3), FilledQty: decimal.Zero, Status: domain.OrderStatusCancelled, CreatedAt: now, UpdatedAt: now},
	}

	for _, o := range orders {
		require.NoError(t, repo.Create(ctx, o))
	}

	result, err := repo.ListAllOpen(ctx)
	require.NoError(t, err)

	assert.Len(t, result, 2)

	ids := make(map[string]bool)
	for _, o := range result {
		ids[o.ID] = true
	}
	assert.True(t, ids["1"], "open order should be included")
	assert.True(t, ids["2"], "partial order should be included")
}

func TestListAllOpen_EmptyRepo(t *testing.T) {
	repo := NewOrderRepository()
	ctx := context.Background()

	result, err := repo.ListAllOpen(ctx)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListAllOpen_MultipleMarkets(t *testing.T) {
	repo := NewOrderRepository()
	ctx := context.Background()
	now := time.Now()

	orders := []*domain.Order{
		{ID: "1", UserID: "u1", MarketID: "m1", OutcomeID: "o1", Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.50), Quantity: decimal.NewFromFloat(10), FilledQty: decimal.Zero, Status: domain.OrderStatusOpen, CreatedAt: now, UpdatedAt: now},
		{ID: "2", UserID: "u2", MarketID: "m2", OutcomeID: "o2", Side: domain.OrderSideSell, Price: decimal.NewFromFloat(0.40), Quantity: decimal.NewFromFloat(8), FilledQty: decimal.Zero, Status: domain.OrderStatusOpen, CreatedAt: now, UpdatedAt: now},
	}

	for _, o := range orders {
		require.NoError(t, repo.Create(ctx, o))
	}

	result, err := repo.ListAllOpen(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)
}
