package service

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// ---------------------------------------------------------------------------
// Tests: CancelAllOrdersByMarket -- Cancels open orders for the target market
// ---------------------------------------------------------------------------

func TestCancelAllOrdersByMarket_CancelsOpenOrders(t *testing.T) {
	svc, orderRepo, userRepo, positionRepo, _, _, engine := newTestOrderService()
	ctx := context.Background()

	// Setup: two users.
	seedUser(userRepo, &domain.User{
		ID:            "user-1",
		WalletAddress: "0x111",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(90),
		LockedBalance: decimal.NewFromInt(10),
		CreatedAt:     time.Now(),
	})
	seedUser(userRepo, &domain.User{
		ID:            "user-2",
		WalletAddress: "0x222",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(95),
		LockedBalance: decimal.NewFromInt(5),
		CreatedAt:     time.Now(),
	})

	// Setup: position for user-2 sell order.
	seedPosition(positionRepo, &domain.Position{
		ID:        "pos-1",
		UserID:    "user-2",
		MarketID:  "market-1",
		OutcomeID: "outcome-a",
		Quantity:  decimal.NewFromInt(5),
		AvgPrice:  decimal.NewFromFloat(0.40),
		UpdatedAt: time.Now(),
	})

	// Setup: 2 open buy orders for market-1.
	buyOrder1 := &domain.Order{
		ID:        "order-buy-1",
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "outcome-a",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.50),
		Quantity:  decimal.NewFromInt(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	buyOrder2 := &domain.Order{
		ID:        "order-buy-2",
		UserID:    "user-2",
		MarketID:  "market-1",
		OutcomeID: "outcome-b",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.30),
		Quantity:  decimal.NewFromInt(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Setup: 1 open sell order for market-1.
	sellOrder1 := &domain.Order{
		ID:        "order-sell-1",
		UserID:    "user-2",
		MarketID:  "market-1",
		OutcomeID: "outcome-a",
		Side:      domain.OrderSideSell,
		Price:     decimal.NewFromFloat(0.60),
		Quantity:  decimal.NewFromInt(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Setup: 1 open order for market-2 (should NOT be cancelled).
	otherMarketOrder := &domain.Order{
		ID:        "order-other",
		UserID:    "user-1",
		MarketID:  "market-2",
		OutcomeID: "outcome-x",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.40),
		Quantity:  decimal.NewFromInt(5),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	seedOrder(orderRepo, buyOrder1)
	seedOrder(orderRepo, buyOrder2)
	seedOrder(orderRepo, sellOrder1)
	seedOrder(orderRepo, otherMarketOrder)

	// Place orders in engine so CancelOrder can find them.
	engine.mu.Lock()
	engine.placed = append(engine.placed, buyOrder1, buyOrder2, sellOrder1, otherMarketOrder)
	engine.mu.Unlock()

	// Action: Cancel all orders for market-1.
	count, err := svc.CancelAllOrdersByMarket(ctx, "market-1")
	require.NoError(t, err)

	// Assert: 3 orders were cancelled.
	assert.Equal(t, int64(3), count, "should cancel 3 open orders in market-1")

	// Assert: All market-1 orders have status cancelled.
	o1, err := orderRepo.GetByID(ctx, "order-buy-1")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusCancelled, o1.Status,
		"buy order 1 should be cancelled")

	o2, err := orderRepo.GetByID(ctx, "order-buy-2")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusCancelled, o2.Status,
		"buy order 2 should be cancelled")

	o3, err := orderRepo.GetByID(ctx, "order-sell-1")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusCancelled, o3.Status,
		"sell order 1 should be cancelled")

	// Assert: market-2 order remains open.
	o4, err := orderRepo.GetByID(ctx, "order-other")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusOpen, o4.Status,
		"market-2 order should remain open")
}

// ---------------------------------------------------------------------------
// Tests: CancelAllOrdersByMarket -- Releases locked funds for buy orders
// ---------------------------------------------------------------------------

func TestCancelAllOrdersByMarket_ReleasesBuyLockedFunds(t *testing.T) {
	svc, orderRepo, userRepo, _, _, _, engine := newTestOrderService()
	ctx := context.Background()

	// Setup: user-1 has balance=95, locked=5 (from a 10@0.50 buy order).
	seedUser(userRepo, &domain.User{
		ID:            "user-1",
		WalletAddress: "0x111",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(95),
		LockedBalance: decimal.NewFromInt(5),
		CreatedAt:     time.Now(),
	})

	// Setup: user-2 has balance=94, locked=6 (from a 20@0.30 buy order).
	seedUser(userRepo, &domain.User{
		ID:            "user-2",
		WalletAddress: "0x222",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(94),
		LockedBalance: decimal.NewFromInt(6),
		CreatedAt:     time.Now(),
	})

	// Setup: open buy order for user-1: 10@0.50, unfilled cost = 10 * 0.50 = 5.
	buyOrder1 := &domain.Order{
		ID:        "order-b1",
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "outcome-a",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.50),
		Quantity:  decimal.NewFromInt(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Setup: open buy order for user-2: 20@0.30, unfilled cost = 20 * 0.30 = 6.
	buyOrder2 := &domain.Order{
		ID:        "order-b2",
		UserID:    "user-2",
		MarketID:  "market-1",
		OutcomeID: "outcome-b",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.30),
		Quantity:  decimal.NewFromInt(20),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	seedOrder(orderRepo, buyOrder1)
	seedOrder(orderRepo, buyOrder2)

	// Place orders in engine so CancelOrder can find them.
	engine.mu.Lock()
	engine.placed = append(engine.placed, buyOrder1, buyOrder2)
	engine.mu.Unlock()

	// Action: Cancel all orders for market-1.
	_, err := svc.CancelAllOrdersByMarket(ctx, "market-1")
	require.NoError(t, err)

	// Assert: user-1 balance increases by 5 (locked funds released).
	user1, err := userRepo.GetByID(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, user1.Balance.Equal(decimal.NewFromInt(100)),
		"user-1 balance should be 100 after releasing 5, got: %s", user1.Balance)
	assert.True(t, user1.LockedBalance.Equal(decimal.Zero),
		"user-1 locked balance should be 0 after releasing 5, got: %s", user1.LockedBalance)

	// Assert: user-2 balance increases by 6 (locked funds released).
	user2, err := userRepo.GetByID(ctx, "user-2")
	require.NoError(t, err)
	assert.True(t, user2.Balance.Equal(decimal.NewFromInt(100)),
		"user-2 balance should be 100 after releasing 6, got: %s", user2.Balance)
	assert.True(t, user2.LockedBalance.Equal(decimal.Zero),
		"user-2 locked balance should be 0 after releasing 6, got: %s", user2.LockedBalance)
}

// ---------------------------------------------------------------------------
// Tests: CancelAllOrdersByMarket -- Restores sell position quantities
// ---------------------------------------------------------------------------

func TestCancelAllOrdersByMarket_RestoresSellPositions(t *testing.T) {
	svc, orderRepo, userRepo, positionRepo, _, _, engine := newTestOrderService()
	ctx := context.Background()

	// Setup: user-1 has a position with qty=5 on outcome-1 (was reduced from
	// 15 to 5 when the sell order for qty=10 was placed).
	seedUser(userRepo, &domain.User{
		ID:            "user-1",
		WalletAddress: "0x111",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})

	seedPosition(positionRepo, &domain.Position{
		ID:        "pos-1",
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "outcome-1",
		Quantity:  decimal.NewFromInt(5),
		AvgPrice:  decimal.NewFromFloat(0.40),
		UpdatedAt: time.Now(),
	})

	// Setup: open sell order for user-1: qty=10 on outcome-1, fully unfilled.
	sellOrder := &domain.Order{
		ID:        "order-s1",
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "outcome-1",
		Side:      domain.OrderSideSell,
		Price:     decimal.NewFromFloat(0.60),
		Quantity:  decimal.NewFromInt(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	seedOrder(orderRepo, sellOrder)

	// Place order in engine so CancelOrder can find it.
	engine.mu.Lock()
	engine.placed = append(engine.placed, sellOrder)
	engine.mu.Unlock()

	// Action: Cancel all orders for market-1.
	count, err := svc.CancelAllOrdersByMarket(ctx, "market-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "should cancel 1 open order")

	// Assert: position quantity restored by unfilled sell qty.
	// Position was 5, sell order was 10 unfilled, so position should be 15.
	pos, err := positionRepo.GetByUserAndOutcome(ctx, "user-1", "outcome-1")
	require.NoError(t, err)
	assert.True(t, pos.Quantity.Equal(decimal.NewFromInt(15)),
		"position quantity should be restored from 5 to 15, got: %s", pos.Quantity)
}

// ---------------------------------------------------------------------------
// Tests: CancelAllOrdersByMarket -- No open orders returns zero
// ---------------------------------------------------------------------------

func TestCancelAllOrdersByMarket_NoOpenOrders_ReturnsZero(t *testing.T) {
	svc, orderRepo, userRepo, _, _, _, _ := newTestOrderService()
	ctx := context.Background()

	// Setup: user with no changes expected.
	seedUser(userRepo, &domain.User{
		ID:            "user-1",
		WalletAddress: "0x111",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})

	// Setup: only filled and cancelled orders for market-1 (no open orders).
	seedOrder(orderRepo, &domain.Order{
		ID:        "order-filled",
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "outcome-a",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.50),
		Quantity:  decimal.NewFromInt(10),
		FilledQty: decimal.NewFromInt(10),
		Status:    domain.OrderStatusFilled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	seedOrder(orderRepo, &domain.Order{
		ID:        "order-cancelled",
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "outcome-b",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.40),
		Quantity:  decimal.NewFromInt(5),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusCancelled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// Action: Cancel all orders for market-1.
	count, err := svc.CancelAllOrdersByMarket(ctx, "market-1")
	require.NoError(t, err)

	// Assert: returns 0 since there are no open orders.
	assert.Equal(t, int64(0), count, "should return 0 when no open orders exist")

	// Assert: user balance remains unchanged.
	user, err := userRepo.GetByID(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, user.Balance.Equal(decimal.NewFromInt(100)),
		"user balance should remain 100, got: %s", user.Balance)
	assert.True(t, user.LockedBalance.Equal(decimal.Zero),
		"user locked balance should remain 0, got: %s", user.LockedBalance)

	// Assert: existing order statuses remain unchanged.
	filledOrder, err := orderRepo.GetByID(ctx, "order-filled")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusFilled, filledOrder.Status,
		"filled order should remain filled")

	cancelledOrder, err := orderRepo.GetByID(ctx, "order-cancelled")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusCancelled, cancelledOrder.Status,
		"already-cancelled order should remain cancelled")
}
