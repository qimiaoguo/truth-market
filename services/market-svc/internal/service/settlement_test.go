package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/eventbus"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// ---------------------------------------------------------------------------
// Mock: UserRepository (settlement-specific)
// ---------------------------------------------------------------------------

type mockSettlementUserRepo struct {
	mu               sync.RWMutex
	users            map[string]*domain.User
	updateBalanceErr error // inject failures
}

func newMockSettlementUserRepo() *mockSettlementUserRepo {
	return &mockSettlementUserRepo{
		users: make(map[string]*domain.User),
	}
}

func (m *mockSettlementUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
	return nil
}

func (m *mockSettlementUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return u, nil
}

func (m *mockSettlementUserRepo) GetByWallet(ctx context.Context, addr string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if u.WalletAddress == addr {
			return u, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockSettlementUserRepo) UpdateBalance(ctx context.Context, id string, balance, locked decimal.Decimal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateBalanceErr != nil {
		return m.updateBalanceErr
	}
	u, ok := m.users[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	u.Balance = balance
	u.LockedBalance = locked
	return nil
}

func (m *mockSettlementUserRepo) List(ctx context.Context, filter repository.UserFilter) ([]*domain.User, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.User
	for _, u := range m.users {
		result = append(result, u)
	}
	return result, int64(len(result)), nil
}

// ---------------------------------------------------------------------------
// Mock: PositionRepository (settlement-specific)
// ---------------------------------------------------------------------------

type mockSettlementPositionRepo struct {
	mu        sync.RWMutex
	positions []*domain.Position
}

func newMockSettlementPositionRepo() *mockSettlementPositionRepo {
	return &mockSettlementPositionRepo{}
}

func (m *mockSettlementPositionRepo) Upsert(ctx context.Context, position *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.positions {
		if p.UserID == position.UserID && p.OutcomeID == position.OutcomeID {
			m.positions[i] = position
			return nil
		}
	}
	m.positions = append(m.positions, position)
	return nil
}

func (m *mockSettlementPositionRepo) GetByUserAndOutcome(ctx context.Context, userID, outcomeID string) (*domain.Position, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.positions {
		if p.UserID == userID && p.OutcomeID == outcomeID {
			return p, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockSettlementPositionRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Position, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Position
	for _, p := range m.positions {
		if p.UserID == userID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockSettlementPositionRepo) ListByMarket(ctx context.Context, marketID string) ([]*domain.Position, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Position
	for _, p := range m.positions {
		if p.MarketID == marketID {
			result = append(result, p)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Mock: OrderCanceller
// ---------------------------------------------------------------------------

type mockOrderCanceller struct {
	mu       sync.Mutex
	called   bool
	marketID string
	count    int64
	err      error
}

func (m *mockOrderCanceller) CancelAllOrdersByMarket(ctx context.Context, marketID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	m.marketID = marketID
	if m.err != nil {
		return 0, m.err
	}
	return m.count, nil
}

// ---------------------------------------------------------------------------
// Mock: EventBus (settlement-specific)
// ---------------------------------------------------------------------------

type publishedEvent struct {
	topic string
	event domain.DomainEvent
}

type mockSettlementEventBus struct {
	mu     sync.Mutex
	events []publishedEvent
}

func (m *mockSettlementEventBus) Publish(ctx context.Context, topic string, event domain.DomainEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, publishedEvent{topic: topic, event: event})
	return nil
}

func (m *mockSettlementEventBus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	return nil
}

func (m *mockSettlementEventBus) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// Test helper: newTestSettlementService
// ---------------------------------------------------------------------------

func newTestSettlementService() (
	*SettlementService,
	*mockMarketRepo,
	*mockOutcomeRepo,
	*mockSettlementPositionRepo,
	*mockSettlementUserRepo,
	*mockOrderCanceller,
	*mockTxManager,
	*mockSettlementEventBus,
) {
	marketRepo := newMockMarketRepo()
	outcomeRepo := newMockOutcomeRepo()
	positionRepo := newMockSettlementPositionRepo()
	userRepo := newMockSettlementUserRepo()
	canceller := &mockOrderCanceller{}
	txManager := &mockTxManager{}
	bus := &mockSettlementEventBus{}

	svc := NewSettlementService(marketRepo, outcomeRepo, positionRepo, userRepo, canceller, txManager, bus)
	return svc, marketRepo, outcomeRepo, positionRepo, userRepo, canceller, txManager, bus
}

// seedPositions inserts positions directly into the mock repo.
func seedPositions(repo *mockSettlementPositionRepo, positions []*domain.Position) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.positions = append(repo.positions, positions...)
}

// seedUsers inserts users directly into the mock repo.
func seedUsers(repo *mockSettlementUserRepo, users []*domain.User) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, u := range users {
		repo.users[u.ID] = u
	}
}

// ---------------------------------------------------------------------------
// Test 1: ResolveMarket sets the winning outcome and market status
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_SetsWinningOutcome(t *testing.T) {
	svc, marketRepo, outcomeRepo, _, _, _, txManager, _ := newTestSettlementService()
	ctx := context.Background()

	// Setup: closed binary market with Yes/No outcomes.
	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Will BTC hit $100k?",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary,
		Category:   "crypto",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	// Reset txCalled to track this specific call.
	txManager.txCalled = false

	err := svc.ResolveMarket(ctx, "m-1", "o-yes")
	require.NoError(t, err)

	// Market status should be resolved with the winning outcome set.
	resolved, err := marketRepo.GetByID(ctx, "m-1")
	require.NoError(t, err)
	assert.Equal(t, domain.MarketStatusResolved, resolved.Status, "market should be resolved")
	require.NotNil(t, resolved.ResolvedOutcomeID, "resolved outcome ID should be set")
	assert.Equal(t, "o-yes", *resolved.ResolvedOutcomeID)

	// The winning outcome should be flagged as winner.
	outcomes, err := outcomeRepo.ListByMarket(ctx, "m-1")
	require.NoError(t, err)
	for _, o := range outcomes {
		if o.ID == "o-yes" {
			assert.True(t, o.IsWinner, "winning outcome should have IsWinner=true")
		} else {
			assert.False(t, o.IsWinner, "non-winning outcome should have IsWinner=false")
		}
	}

	// Resolution should happen within a transaction.
	assert.True(t, txManager.txCalled, "resolution should happen within a transaction")
}

// ---------------------------------------------------------------------------
// Test 2: Winner gets $1 per token held
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_WinnerGets1USDPerToken(t *testing.T) {
	svc, marketRepo, outcomeRepo, positionRepo, userRepo, _, _, _ := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Will ETH flip BTC?",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary,
		Category:   "crypto",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	// user-1 holds 10 Yes tokens, user-2 holds 5 Yes tokens.
	seedUsers(userRepo, []*domain.User{
		{ID: "user-1", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
		{ID: "user-2", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
	})
	seedPositions(positionRepo, []*domain.Position{
		{ID: "p-1", UserID: "user-1", MarketID: "m-1", OutcomeID: "o-yes", Quantity: decimal.NewFromInt(10), AvgPrice: decimal.NewFromFloat(0.60), UpdatedAt: now},
		{ID: "p-2", UserID: "user-2", MarketID: "m-1", OutcomeID: "o-yes", Quantity: decimal.NewFromInt(5), AvgPrice: decimal.NewFromFloat(0.55), UpdatedAt: now},
	})

	err := svc.ResolveMarket(ctx, "m-1", "o-yes")
	require.NoError(t, err)

	// user-1 should receive $10 (10 tokens * $1), user-2 should receive $5 (5 tokens * $1).
	u1, err := userRepo.GetByID(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, u1.Balance.Equal(decimal.NewFromInt(10)),
		"user-1 balance should be 10, got %s", u1.Balance)

	u2, err := userRepo.GetByID(ctx, "user-2")
	require.NoError(t, err)
	assert.True(t, u2.Balance.Equal(decimal.NewFromInt(5)),
		"user-2 balance should be 5, got %s", u2.Balance)
}

// ---------------------------------------------------------------------------
// Test 3: Loser tokens are worth $0
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_LoserTokensBecome0(t *testing.T) {
	svc, marketRepo, outcomeRepo, positionRepo, userRepo, _, _, _ := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Will it snow in July?",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary,
		Category:   "weather",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	// user-1 holds 10 Yes tokens (winner), user-2 holds 20 No tokens (loser).
	initialBalanceUser2 := decimal.NewFromInt(100)
	seedUsers(userRepo, []*domain.User{
		{ID: "user-1", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
		{ID: "user-2", Balance: initialBalanceUser2, LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
	})
	seedPositions(positionRepo, []*domain.Position{
		{ID: "p-1", UserID: "user-1", MarketID: "m-1", OutcomeID: "o-yes", Quantity: decimal.NewFromInt(10), AvgPrice: decimal.NewFromFloat(0.60), UpdatedAt: now},
		{ID: "p-2", UserID: "user-2", MarketID: "m-1", OutcomeID: "o-no", Quantity: decimal.NewFromInt(20), AvgPrice: decimal.NewFromFloat(0.40), UpdatedAt: now},
	})

	err := svc.ResolveMarket(ctx, "m-1", "o-yes")
	require.NoError(t, err)

	// user-1 wins: gets $10 payout.
	u1, err := userRepo.GetByID(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, u1.Balance.Equal(decimal.NewFromInt(10)),
		"user-1 (winner) balance should be 10, got %s", u1.Balance)

	// user-2 loses: balance should remain unchanged (no payout for loser tokens).
	u2, err := userRepo.GetByID(ctx, "user-2")
	require.NoError(t, err)
	assert.True(t, u2.Balance.Equal(initialBalanceUser2),
		"user-2 (loser) balance should remain %s, got %s", initialBalanceUser2, u2.Balance)
}

// ---------------------------------------------------------------------------
// Test 4: Resolution cancels all open orders
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_CancelsAllOpenOrders(t *testing.T) {
	svc, marketRepo, outcomeRepo, _, _, canceller, _, _ := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Will Mars be colonised by 2030?",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary,
		Category:   "science",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	// Configure the canceller to report it cancelled 3 orders.
	canceller.count = 3

	err := svc.ResolveMarket(ctx, "m-1", "o-yes")
	require.NoError(t, err)

	// OrderCanceller should have been called with the correct market ID.
	canceller.mu.Lock()
	defer canceller.mu.Unlock()
	assert.True(t, canceller.called, "CancelAllOrdersByMarket should have been called")
	assert.Equal(t, "m-1", canceller.marketID, "canceller should be called with market ID m-1")
}

// ---------------------------------------------------------------------------
// Test 5: Resolution releases locked funds via order canceller
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_ReleasesLockedFunds(t *testing.T) {
	svc, marketRepo, outcomeRepo, _, _, canceller, _, _ := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Will fusion power be achieved?",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary,
		Category:   "science",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	// Mock the order canceller to return a count > 0, indicating it cancelled orders
	// and released locked funds.
	canceller.count = 5

	err := svc.ResolveMarket(ctx, "m-1", "o-yes")
	require.NoError(t, err)

	// Verify the canceller was invoked (the integration point for releasing locked funds).
	canceller.mu.Lock()
	defer canceller.mu.Unlock()
	assert.True(t, canceller.called, "CancelAllOrdersByMarket should be called to release locked funds")
	assert.Equal(t, "m-1", canceller.marketID)
}

// ---------------------------------------------------------------------------
// Test 6: Resolving an already-resolved market returns error
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_AlreadyResolved_ReturnsError(t *testing.T) {
	svc, marketRepo, outcomeRepo, _, _, canceller, txManager, _ := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	winnerID := "o-yes"
	seedMarket(marketRepo, &domain.Market{
		ID:                "m-1",
		Title:             "Already resolved market",
		Status:            domain.MarketStatusResolved,
		MarketType:        domain.MarketTypeBinary,
		Category:          "general",
		CreatorID:         "creator-1",
		ResolvedOutcomeID: &winnerID,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0, IsWinner: true},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	// Reset to track that no side effects occur.
	txManager.txCalled = false

	err := svc.ResolveMarket(ctx, "m-1", "o-yes")

	assert.Error(t, err, "resolving an already-resolved market should return an error")
	assert.True(t, apperrors.IsBadRequest(err),
		"error should be BAD_REQUEST, got: %v", err)

	// No state changes should have occurred.
	canceller.mu.Lock()
	assert.False(t, canceller.called, "order canceller should not be called for already-resolved market")
	canceller.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Test 7: Multi-outcome market with one winner
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_MultiOutcome_OneWinner(t *testing.T) {
	svc, marketRepo, outcomeRepo, positionRepo, userRepo, _, _, _ := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Which colour wins the championship?",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeMulti,
		Category:   "sports",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-red", MarketID: "m-1", Label: "Red", Index: 0},
		{ID: "o-blue", MarketID: "m-1", Label: "Blue", Index: 1},
		{ID: "o-green", MarketID: "m-1", Label: "Green", Index: 2},
	})

	// user-1 has Red tokens (10), user-2 has Blue tokens (5), user-3 has Green tokens (8).
	seedUsers(userRepo, []*domain.User{
		{ID: "user-1", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
		{ID: "user-2", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
		{ID: "user-3", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
	})
	seedPositions(positionRepo, []*domain.Position{
		{ID: "p-1", UserID: "user-1", MarketID: "m-1", OutcomeID: "o-red", Quantity: decimal.NewFromInt(10), AvgPrice: decimal.NewFromFloat(0.40), UpdatedAt: now},
		{ID: "p-2", UserID: "user-2", MarketID: "m-1", OutcomeID: "o-blue", Quantity: decimal.NewFromInt(5), AvgPrice: decimal.NewFromFloat(0.30), UpdatedAt: now},
		{ID: "p-3", UserID: "user-3", MarketID: "m-1", OutcomeID: "o-green", Quantity: decimal.NewFromInt(8), AvgPrice: decimal.NewFromFloat(0.30), UpdatedAt: now},
	})

	// Resolve with Red winning.
	err := svc.ResolveMarket(ctx, "m-1", "o-red")
	require.NoError(t, err)

	// user-1 (Red holder) gets $10, user-2 (Blue) gets $0, user-3 (Green) gets $0.
	u1, err := userRepo.GetByID(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, u1.Balance.Equal(decimal.NewFromInt(10)),
		"user-1 (Red winner) balance should be 10, got %s", u1.Balance)

	u2, err := userRepo.GetByID(ctx, "user-2")
	require.NoError(t, err)
	assert.True(t, u2.Balance.Equal(decimal.NewFromInt(0)),
		"user-2 (Blue loser) balance should be 0, got %s", u2.Balance)

	u3, err := userRepo.GetByID(ctx, "user-3")
	require.NoError(t, err)
	assert.True(t, u3.Balance.Equal(decimal.NewFromInt(0)),
		"user-3 (Green loser) balance should be 0, got %s", u3.Balance)
}

// ---------------------------------------------------------------------------
// Test 8: Transaction rollback on partial failure
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_TxRollback_OnPartialFailure(t *testing.T) {
	svc, marketRepo, outcomeRepo, positionRepo, userRepo, _, txManager, _ := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Rollback test market",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary,
		Category:   "general",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	seedUsers(userRepo, []*domain.User{
		{ID: "user-1", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
	})
	seedPositions(positionRepo, []*domain.Position{
		{ID: "p-1", UserID: "user-1", MarketID: "m-1", OutcomeID: "o-yes", Quantity: decimal.NewFromInt(10), AvgPrice: decimal.NewFromFloat(0.60), UpdatedAt: now},
	})

	// Inject failure: UpdateBalance will return an error.
	userRepo.updateBalanceErr = apperrors.New("INTERNAL_ERROR", "database write failure")

	// Reset to track transactional behaviour.
	txManager.txCalled = false

	err := svc.ResolveMarket(ctx, "m-1", "o-yes")

	assert.Error(t, err, "resolution should fail when UpdateBalance errors")
	assert.True(t, txManager.txCalled, "txManager.WithTx should have been invoked")
}

// ---------------------------------------------------------------------------
// Test 9: CancelMarket refunds all minted tokens at avg price
// ---------------------------------------------------------------------------

func TestSettlement_CancelMarket_RefundsAllMintedTokens(t *testing.T) {
	svc, marketRepo, outcomeRepo, positionRepo, userRepo, canceller, _, _ := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Cancellable market",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary,
		Category:   "general",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	// user-1 holds 10 Yes tokens at avgPrice=0.60, user-2 holds 5 No tokens at avgPrice=0.40.
	seedUsers(userRepo, []*domain.User{
		{ID: "user-1", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
		{ID: "user-2", Balance: decimal.NewFromInt(0), LockedBalance: decimal.NewFromInt(0), CreatedAt: now},
	})
	seedPositions(positionRepo, []*domain.Position{
		{ID: "p-1", UserID: "user-1", MarketID: "m-1", OutcomeID: "o-yes", Quantity: decimal.NewFromInt(10), AvgPrice: decimal.NewFromFloat(0.60), UpdatedAt: now},
		{ID: "p-2", UserID: "user-2", MarketID: "m-1", OutcomeID: "o-no", Quantity: decimal.NewFromInt(5), AvgPrice: decimal.NewFromFloat(0.40), UpdatedAt: now},
	})

	canceller.count = 2

	err := svc.CancelMarket(ctx, "m-1")
	require.NoError(t, err)

	// Market status should be cancelled.
	cancelled, err := marketRepo.GetByID(ctx, "m-1")
	require.NoError(t, err)
	assert.Equal(t, domain.MarketStatusCancelled, cancelled.Status, "market should be cancelled")

	// user-1 refund: 10 * 0.60 = $6.
	u1, err := userRepo.GetByID(ctx, "user-1")
	require.NoError(t, err)
	expectedU1 := decimal.NewFromFloat(6.0)
	assert.True(t, u1.Balance.Equal(expectedU1),
		"user-1 should be refunded $6.00, got %s", u1.Balance)

	// user-2 refund: 5 * 0.40 = $2.
	u2, err := userRepo.GetByID(ctx, "user-2")
	require.NoError(t, err)
	expectedU2 := decimal.NewFromFloat(2.0)
	assert.True(t, u2.Balance.Equal(expectedU2),
		"user-2 should be refunded $2.00, got %s", u2.Balance)

	// OrderCanceller should have been invoked.
	canceller.mu.Lock()
	defer canceller.mu.Unlock()
	assert.True(t, canceller.called, "CancelAllOrdersByMarket should have been called on cancel")
	assert.Equal(t, "m-1", canceller.marketID)
}

// ---------------------------------------------------------------------------
// Test 10: ResolveMarket publishes a MarketResolved event
// ---------------------------------------------------------------------------

func TestSettlement_ResolveMarket_PublishesMarketResolvedEvent(t *testing.T) {
	svc, marketRepo, outcomeRepo, _, _, _, _, bus := newTestSettlementService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-1",
		Title:      "Event publishing test",
		Status:     domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary,
		Category:   "general",
		CreatorID:  "creator-1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
	})

	err := svc.ResolveMarket(ctx, "m-1", "o-yes")
	require.NoError(t, err)

	// Verify that an event was published to the MarketResolved topic.
	bus.mu.Lock()
	defer bus.mu.Unlock()

	require.NotEmpty(t, bus.events, "at least one event should have been published")

	var found bool
	for _, pe := range bus.events {
		if pe.topic == eventbus.TopicMarketResolved {
			found = true

			// The event type should be EventMarketResolved.
			assert.Equal(t, domain.EventMarketResolved, pe.event.Type,
				"event type should be market.resolved")

			// Payload should contain market_id and winning_outcome_id.
			var payload map[string]interface{}
			err := json.Unmarshal(pe.event.Payload, &payload)
			require.NoError(t, err, "event payload should be valid JSON")

			assert.Equal(t, "m-1", payload["market_id"],
				"event payload should contain market_id=m-1")
			assert.Equal(t, "o-yes", payload["winning_outcome_id"],
				"event payload should contain winning_outcome_id=o-yes")

			break
		}
	}
	assert.True(t, found, "a MarketResolved event should have been published to topic %s", eventbus.TopicMarketResolved)
}
