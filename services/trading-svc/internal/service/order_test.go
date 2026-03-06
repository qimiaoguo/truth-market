package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
)

// ---------------------------------------------------------------------------
// Mock: OrderRepository
// ---------------------------------------------------------------------------

type mockOrderRepo struct {
	mu     sync.RWMutex
	orders map[string]*domain.Order
}

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{
		orders: make(map[string]*domain.Order),
	}
}

func (m *mockOrderRepo) Create(ctx context.Context, order *domain.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrderRepo) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orders[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return o, nil
}

func (m *mockOrderRepo) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, filled decimal.Decimal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	o.Status = status
	o.FilledQty = filled
	o.UpdatedAt = time.Now()
	return nil
}

func (m *mockOrderRepo) ListOpenByMarket(ctx context.Context, marketID string) ([]*domain.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Order
	for _, o := range m.orders {
		if o.MarketID == marketID && o.Status == domain.OrderStatusOpen {
			result = append(result, o)
		}
	}
	return result, nil
}

func (m *mockOrderRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Order, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Order
	for _, o := range m.orders {
		if o.UserID == userID {
			result = append(result, o)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockOrderRepo) CancelAllByMarket(ctx context.Context, marketID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, o := range m.orders {
		if o.MarketID == marketID && o.Status == domain.OrderStatusOpen {
			o.Status = domain.OrderStatusCancelled
			count++
		}
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Mock: MatchingEngine
// ---------------------------------------------------------------------------

type mockMatchingEngine struct {
	mu     sync.Mutex
	placed []*domain.Order
	// matchResult is returned by PlaceOrder when set.
	matchResult *MatchResult
}

func newMockMatchingEngine() *mockMatchingEngine {
	return &mockMatchingEngine{
		matchResult: &MatchResult{
			Trades:  nil,
			Resting: nil,
		},
	}
}

func (m *mockMatchingEngine) PlaceOrder(order *domain.Order) (*MatchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.placed = append(m.placed, order)

	result := m.matchResult
	if result == nil {
		result = &MatchResult{}
	}
	// If no resting order specified, the order rests on the book.
	if result.Resting == nil {
		result.Resting = order
	}
	return result, nil
}

func (m *mockMatchingEngine) CancelOrder(outcomeID, orderID string) (*domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a cancelled version of the order if it was previously placed.
	for _, o := range m.placed {
		if o.ID == orderID && o.OutcomeID == outcomeID {
			o.Status = domain.OrderStatusCancelled
			return o, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// seedOrder inserts an order directly into the mock order repo.
func seedOrder(repo *mockOrderRepo, order *domain.Order) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.orders[order.ID] = order
}

// seedPosition inserts a position directly into the mock position repo.
func seedPosition(repo *mockPositionRepo, position *domain.Position) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.positions = append(repo.positions, position)
}

// newTestOrderService creates an OrderService wired with in-memory mocks.
func newTestOrderService() (
	*OrderService,
	*mockOrderRepo,
	*mockUserRepo,
	*mockPositionRepo,
	*mockTradeRepo,
	*mockTxManager,
	*mockMatchingEngine,
) {
	orderRepo := newMockOrderRepo()
	userRepo := newMockUserRepo()
	positionRepo := newMockPositionRepo()
	tradeRepo := newMockTradeRepo()
	txManager := &mockTxManager{}
	engine := newMockMatchingEngine()

	svc := NewOrderService(orderRepo, userRepo, positionRepo, tradeRepo, txManager, engine)
	return svc, orderRepo, userRepo, positionRepo, tradeRepo, txManager, engine
}

// ---------------------------------------------------------------------------
// Tests: PlaceOrder -- Buy locks balance funds
// ---------------------------------------------------------------------------

func TestPlaceOrder_BuyLocksBalanceFunds(t *testing.T) {
	svc, orderRepo, userRepo, _, _, txManager, engine := newTestOrderService()
	ctx := context.Background()

	// Setup: user with balance 100.
	seedUser(userRepo, &domain.User{
		ID:            "user-1",
		WalletAddress: "0xabc",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})

	// Action: Place buy order for 10@0.50 (cost = 10 * 0.50 = 5.00).
	price := decimal.NewFromFloat(0.50)
	quantity := decimal.NewFromInt(10)

	order, trades, err := svc.PlaceOrder(ctx, PlaceOrderRequest{
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "o-yes",
		Side:      domain.OrderSideBuy,
		Price:     price,
		Quantity:  quantity,
	})
	require.NoError(t, err)

	// Assert: Order created.
	assert.NotNil(t, order, "order should be returned")
	assert.NotEmpty(t, order.ID, "order ID should be assigned")
	assert.Equal(t, "user-1", order.UserID)
	assert.Equal(t, "market-1", order.MarketID)
	assert.Equal(t, "o-yes", order.OutcomeID)
	assert.Equal(t, domain.OrderSideBuy, order.Side)
	assert.True(t, order.Price.Equal(price), "order price should be 0.50")
	assert.True(t, order.Quantity.Equal(quantity), "order quantity should be 10")
	assert.Equal(t, domain.OrderStatusOpen, order.Status, "new order should have status open")

	// Assert: no immediate trades (resting order).
	assert.Empty(t, trades, "no trades should occur for a resting order")

	// Assert: User locked_balance increases by 5.00, balance decreases by 5.00.
	user, err := userRepo.GetByID(ctx, "user-1")
	require.NoError(t, err)

	expectedCost := price.Mul(quantity) // 0.50 * 10 = 5.00
	expectedBalance := decimal.NewFromInt(100).Sub(expectedCost)
	assert.True(t, user.Balance.Equal(expectedBalance),
		"user balance should be 95.00, got: %s", user.Balance)
	assert.True(t, user.LockedBalance.Equal(expectedCost),
		"user locked_balance should be 5.00, got: %s", user.LockedBalance)

	// Assert: Order persisted in repo.
	orderRepo.mu.RLock()
	assert.Len(t, orderRepo.orders, 1, "one order should be persisted")
	orderRepo.mu.RUnlock()

	// Assert: Order sent to matching engine.
	engine.mu.Lock()
	assert.Len(t, engine.placed, 1, "order should be sent to matching engine")
	engine.mu.Unlock()

	// Assert: transactional.
	assert.True(t, txManager.txCalled, "order placement should happen within a transaction")
}

// ---------------------------------------------------------------------------
// Tests: PlaceOrder -- Sell locks position
// ---------------------------------------------------------------------------

func TestPlaceOrder_SellLocksPosition(t *testing.T) {
	svc, orderRepo, userRepo, positionRepo, _, _, engine := newTestOrderService()
	ctx := context.Background()

	// Setup: user with a position qty=20 on outcome-1.
	seedUser(userRepo, &domain.User{
		ID:            "user-2",
		WalletAddress: "0xdef",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(50),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})
	seedPosition(positionRepo, &domain.Position{
		ID:        "pos-1",
		UserID:    "user-2",
		MarketID:  "market-1",
		OutcomeID: "o-1",
		Quantity:  decimal.NewFromInt(20),
		AvgPrice:  decimal.NewFromFloat(0.40),
		UpdatedAt: time.Now(),
	})

	// Action: Place sell order for 10@0.60.
	price := decimal.NewFromFloat(0.60)
	quantity := decimal.NewFromInt(10)

	order, _, err := svc.PlaceOrder(ctx, PlaceOrderRequest{
		UserID:    "user-2",
		MarketID:  "market-1",
		OutcomeID: "o-1",
		Side:      domain.OrderSideSell,
		Price:     price,
		Quantity:  quantity,
	})
	require.NoError(t, err)

	// Assert: Order created.
	assert.NotNil(t, order, "order should be returned")
	assert.Equal(t, domain.OrderSideSell, order.Side)
	assert.True(t, order.Price.Equal(price))
	assert.True(t, order.Quantity.Equal(quantity))

	// Assert: Position quantity decreases by 10 (locked for the sell order).
	pos, err := positionRepo.GetByUserAndOutcome(ctx, "user-2", "o-1")
	require.NoError(t, err)
	assert.True(t, pos.Quantity.Equal(decimal.NewFromInt(10)),
		"position quantity should decrease from 20 to 10 after locking shares for sell, got: %s", pos.Quantity)

	// Assert: Order persisted in repo.
	orderRepo.mu.RLock()
	assert.Len(t, orderRepo.orders, 1, "one order should be persisted")
	orderRepo.mu.RUnlock()

	// Assert: Order sent to matching engine.
	engine.mu.Lock()
	assert.Len(t, engine.placed, 1, "order should be sent to matching engine")
	engine.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Tests: CancelOrder -- Releases locked funds
// ---------------------------------------------------------------------------

func TestCancelOrder_ReleasesLockedFunds(t *testing.T) {
	svc, orderRepo, userRepo, _, _, _, engine := newTestOrderService()
	ctx := context.Background()

	// Setup: user has an open buy order 10@0.50, locked_balance=5.00.
	seedUser(userRepo, &domain.User{
		ID:            "user-3",
		WalletAddress: "0x333",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(95),
		LockedBalance: decimal.NewFromFloat(5.00),
		CreatedAt:     time.Now(),
	})

	openOrder := &domain.Order{
		ID:        "order-1",
		UserID:    "user-3",
		MarketID:  "market-1",
		OutcomeID: "o-yes",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.50),
		Quantity:  decimal.NewFromInt(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	seedOrder(orderRepo, openOrder)

	// Also place the order in the engine so CancelOrder can find it.
	engine.mu.Lock()
	engine.placed = append(engine.placed, openOrder)
	engine.mu.Unlock()

	// Action: Cancel the order.
	err := svc.CancelOrder(ctx, "user-3", "order-1")
	require.NoError(t, err)

	// Assert: Order status -> cancelled.
	order, err := orderRepo.GetByID(ctx, "order-1")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusCancelled, order.Status,
		"order should be cancelled after CancelOrder")

	// Assert: User locked_balance decreases by 5.00, balance increases by 5.00.
	user, err := userRepo.GetByID(ctx, "user-3")
	require.NoError(t, err)

	// The unfilled cost: (quantity - filledQty) * price = (10 - 0) * 0.50 = 5.00
	assert.True(t, user.Balance.Equal(decimal.NewFromInt(100)),
		"user balance should be restored to 100, got: %s", user.Balance)
	assert.True(t, user.LockedBalance.Equal(decimal.Zero),
		"user locked_balance should be 0 after cancel, got: %s", user.LockedBalance)
}

// ---------------------------------------------------------------------------
// Tests: CancelOrder -- Not owner returns error
// ---------------------------------------------------------------------------

func TestCancelOrder_NotOwner_ReturnsError(t *testing.T) {
	svc, orderRepo, userRepo, _, _, _, _ := newTestOrderService()
	ctx := context.Background()

	// Setup: order belongs to user-A.
	seedUser(userRepo, &domain.User{
		ID:            "user-A",
		WalletAddress: "0xAAA",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.NewFromFloat(5.00),
		CreatedAt:     time.Now(),
	})
	seedUser(userRepo, &domain.User{
		ID:            "user-B",
		WalletAddress: "0xBBB",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})
	seedOrder(orderRepo, &domain.Order{
		ID:        "order-2",
		UserID:    "user-A",
		MarketID:  "market-1",
		OutcomeID: "o-yes",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.50),
		Quantity:  decimal.NewFromInt(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// Action: user-B tries to cancel user-A's order.
	err := svc.CancelOrder(ctx, "user-B", "order-2")

	// Assert: FORBIDDEN error.
	assert.Error(t, err, "non-owner should not be able to cancel another user's order")
	assert.True(t, apperrors.IsForbidden(err),
		"error should be FORBIDDEN, got: %v", err)

	// Assert: Order status remains open.
	order, err := orderRepo.GetByID(ctx, "order-2")
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusOpen, order.Status,
		"order should remain open after failed cancel attempt")
}

// ---------------------------------------------------------------------------
// Tests: PlaceOrder -- Invalid price returns error
// ---------------------------------------------------------------------------

func TestPlaceOrder_InvalidPrice_ReturnsError(t *testing.T) {
	svc, orderRepo, userRepo, _, _, _, engine := newTestOrderService()
	ctx := context.Background()

	// Setup: user with sufficient balance.
	seedUser(userRepo, &domain.User{
		ID:            "user-5",
		WalletAddress: "0x555",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})

	// Test price above 0.99.
	order, trades, err := svc.PlaceOrder(ctx, PlaceOrderRequest{
		UserID:    "user-5",
		MarketID:  "market-1",
		OutcomeID: "o-yes",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(1.50),
		Quantity:  decimal.NewFromInt(10),
	})

	assert.Error(t, err, "price 1.50 should be rejected")
	assert.True(t, apperrors.IsInvalidPrice(err),
		"error should be INVALID_PRICE, got: %v", err)
	assert.Nil(t, order, "no order should be returned on invalid price")
	assert.Nil(t, trades, "no trades should be returned on invalid price")

	// Test price at 0 (below 0.01).
	order2, trades2, err2 := svc.PlaceOrder(ctx, PlaceOrderRequest{
		UserID:    "user-5",
		MarketID:  "market-1",
		OutcomeID: "o-yes",
		Side:      domain.OrderSideBuy,
		Price:     decimal.Zero,
		Quantity:  decimal.NewFromInt(10),
	})

	assert.Error(t, err2, "price 0 should be rejected")
	assert.True(t, apperrors.IsInvalidPrice(err2),
		"error should be INVALID_PRICE for zero price, got: %v", err2)
	assert.Nil(t, order2, "no order should be returned on invalid price")
	assert.Nil(t, trades2, "no trades should be returned on invalid price")

	// Test price exactly at 1.00 (above 0.99).
	order3, trades3, err3 := svc.PlaceOrder(ctx, PlaceOrderRequest{
		UserID:    "user-5",
		MarketID:  "market-1",
		OutcomeID: "o-yes",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromInt(1),
		Quantity:  decimal.NewFromInt(10),
	})

	assert.Error(t, err3, "price 1.00 should be rejected")
	assert.True(t, apperrors.IsInvalidPrice(err3),
		"error should be INVALID_PRICE for price 1.00, got: %v", err3)
	assert.Nil(t, order3, "no order should be returned on invalid price")
	assert.Nil(t, trades3, "no trades should be returned on invalid price")

	// Assert: no orders should have been created.
	orderRepo.mu.RLock()
	assert.Empty(t, orderRepo.orders, "no orders should be persisted for invalid prices")
	orderRepo.mu.RUnlock()

	// Assert: nothing sent to matching engine.
	engine.mu.Lock()
	assert.Empty(t, engine.placed, "no orders should be sent to matching engine for invalid prices")
	engine.mu.Unlock()

	// Assert: user balance unchanged.
	user, err := userRepo.GetByID(ctx, "user-5")
	require.NoError(t, err)
	assert.True(t, user.Balance.Equal(decimal.NewFromInt(100)),
		"user balance should remain 100, got: %s", user.Balance)
	assert.True(t, user.LockedBalance.Equal(decimal.Zero),
		"user locked_balance should remain 0, got: %s", user.LockedBalance)
}
