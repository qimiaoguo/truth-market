package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// ---------------------------------------------------------------------------
// Mock: TxManager (shared across test files in this package)
// ---------------------------------------------------------------------------

// mockTxManager executes the supplied function directly (no real transaction).
// txCalled tracks whether WithTx was invoked so tests can assert transactional
// behaviour. If txErr is set, WithTx returns it instead of calling fn.
type mockTxManager struct {
	txCalled bool
	txErr    error
}

func (m *mockTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	m.txCalled = true
	if m.txErr != nil {
		return m.txErr
	}
	return fn(ctx)
}

// ---------------------------------------------------------------------------
// Mock: UserRepository
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	mu    sync.RWMutex
	users map[string]*domain.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*domain.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByWallet(ctx context.Context, addr string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if u.WalletAddress == addr {
			return u, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockUserRepo) UpdateBalance(ctx context.Context, id string, balance, locked decimal.Decimal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	u.Balance = balance
	u.LockedBalance = locked
	return nil
}

func (m *mockUserRepo) List(ctx context.Context, filter repository.UserFilter) ([]*domain.User, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.User
	for _, u := range m.users {
		result = append(result, u)
	}
	return result, int64(len(result)), nil
}

// ---------------------------------------------------------------------------
// Mock: OutcomeRepository
// ---------------------------------------------------------------------------

type mockOutcomeRepo struct {
	mu       sync.RWMutex
	outcomes map[string][]*domain.Outcome // marketID -> outcomes
}

func newMockOutcomeRepo() *mockOutcomeRepo {
	return &mockOutcomeRepo{
		outcomes: make(map[string][]*domain.Outcome),
	}
}

func (m *mockOutcomeRepo) CreateBatch(ctx context.Context, outcomes []*domain.Outcome) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range outcomes {
		m.outcomes[o.MarketID] = append(m.outcomes[o.MarketID], o)
	}
	return nil
}

func (m *mockOutcomeRepo) ListByMarket(ctx context.Context, marketID string) ([]*domain.Outcome, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ocs, ok := m.outcomes[marketID]
	if !ok {
		return nil, nil
	}
	return ocs, nil
}

func (m *mockOutcomeRepo) SetWinner(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ocs := range m.outcomes {
		for _, o := range ocs {
			if o.ID == id {
				o.IsWinner = true
				return nil
			}
		}
	}
	return apperrors.ErrNotFound
}

// ---------------------------------------------------------------------------
// Mock: PositionRepository
// ---------------------------------------------------------------------------

type mockPositionRepo struct {
	mu        sync.RWMutex
	positions []*domain.Position
	// upsertErr lets tests inject failures for specific calls.
	upsertErr     error
	upsertErrOnce bool   // if true, only the next Upsert call fails
	upsertCount   int    // tracks how many times Upsert has been called
	failOnCall    int    // which call number should fail (1-indexed, 0 = disabled)
}

func newMockPositionRepo() *mockPositionRepo {
	return &mockPositionRepo{}
}

func (m *mockPositionRepo) Upsert(ctx context.Context, position *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCount++

	// Fail on a specific call number if configured.
	if m.failOnCall > 0 && m.upsertCount == m.failOnCall {
		return m.upsertErr
	}

	// General failure mode.
	if m.upsertErr != nil && m.failOnCall == 0 {
		return m.upsertErr
	}

	// Update existing position or append new one.
	for i, p := range m.positions {
		if p.UserID == position.UserID && p.OutcomeID == position.OutcomeID {
			m.positions[i] = position
			return nil
		}
	}
	m.positions = append(m.positions, position)
	return nil
}

func (m *mockPositionRepo) GetByUserAndOutcome(ctx context.Context, userID, outcomeID string) (*domain.Position, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.positions {
		if p.UserID == userID && p.OutcomeID == outcomeID {
			return p, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockPositionRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Position, error) {
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

func (m *mockPositionRepo) ListByMarket(ctx context.Context, marketID string) ([]*domain.Position, error) {
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
// Mock: TradeRepository
// ---------------------------------------------------------------------------

type mockTradeRepo struct {
	mu      sync.RWMutex
	trades  []*domain.Trade
	mintTxs []*domain.MintTransaction
}

func newMockTradeRepo() *mockTradeRepo {
	return &mockTradeRepo{}
}

func (m *mockTradeRepo) Create(ctx context.Context, trade *domain.Trade) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trades = append(m.trades, trade)
	return nil
}

func (m *mockTradeRepo) ListByMarket(ctx context.Context, marketID string, limit, offset int) ([]*domain.Trade, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Trade
	for _, t := range m.trades {
		if t.MarketID == marketID {
			result = append(result, t)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockTradeRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Trade, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Trade
	for _, t := range m.trades {
		if t.MakerUserID == userID || t.TakerUserID == userID {
			result = append(result, t)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockTradeRepo) CreateMintTx(ctx context.Context, tx *domain.MintTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mintTxs = append(m.mintTxs, tx)
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// seedUser inserts a user directly into the mock repo.
func seedUser(repo *mockUserRepo, user *domain.User) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.users[user.ID] = user
}

// seedOutcomesForMint inserts outcomes directly into the mock outcome repo.
func seedOutcomesForMint(repo *mockOutcomeRepo, outcomes []*domain.Outcome) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, o := range outcomes {
		repo.outcomes[o.MarketID] = append(repo.outcomes[o.MarketID], o)
	}
}

// newTestMintService creates a MintService wired with in-memory mocks.
func newTestMintService() (*MintService, *mockUserRepo, *mockOutcomeRepo, *mockPositionRepo, *mockTradeRepo, *mockTxManager) {
	userRepo := newMockUserRepo()
	outcomeRepo := newMockOutcomeRepo()
	positionRepo := newMockPositionRepo()
	tradeRepo := newMockTradeRepo()
	txManager := &mockTxManager{}

	svc := NewMintService(userRepo, outcomeRepo, positionRepo, tradeRepo, txManager)
	return svc, userRepo, outcomeRepo, positionRepo, tradeRepo, txManager
}

// ---------------------------------------------------------------------------
// Tests: MintTokens -- Binary market creates 2 positions
// ---------------------------------------------------------------------------

func TestMintTokens_Binary_Creates2Positions(t *testing.T) {
	svc, userRepo, outcomeRepo, positionRepo, tradeRepo, txManager := newTestMintService()
	ctx := context.Background()

	// Setup: user with balance 100, binary market with Yes/No outcomes.
	seedUser(userRepo, &domain.User{
		ID:            "user-1",
		WalletAddress: "0xabc",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})
	seedOutcomesForMint(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "market-1", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "market-1", Label: "No", Index: 1},
	})

	// Action: Mint 10 tokens.
	quantity := decimal.NewFromInt(10)
	positions, err := svc.MintTokens(ctx, "user-1", "market-1", quantity)
	require.NoError(t, err)

	// Assert: 2 positions created (Yes qty=10, No qty=10).
	require.Len(t, positions, 2, "binary mint should produce 2 positions")

	foundYes := false
	foundNo := false
	for _, p := range positions {
		assert.Equal(t, "user-1", p.UserID, "position should belong to the requesting user")
		assert.Equal(t, "market-1", p.MarketID, "position should reference the correct market")
		assert.True(t, p.Quantity.Equal(quantity), "each position quantity should equal the minted amount")
		if p.OutcomeID == "o-yes" {
			foundYes = true
		}
		if p.OutcomeID == "o-no" {
			foundNo = true
		}
	}
	assert.True(t, foundYes, "should have a position for the Yes outcome")
	assert.True(t, foundNo, "should have a position for the No outcome")

	// Assert: User balance deducted by 10 (cost = quantity * 1 USDT per complete set).
	user, err := userRepo.GetByID(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, user.Balance.Equal(decimal.NewFromInt(90)),
		"user balance should be 90 after minting 10, got: %s", user.Balance)

	// Assert: MintTransaction recorded.
	tradeRepo.mu.RLock()
	require.Len(t, tradeRepo.mintTxs, 1, "one mint transaction should be recorded")
	mintTx := tradeRepo.mintTxs[0]
	tradeRepo.mu.RUnlock()

	assert.Equal(t, "user-1", mintTx.UserID)
	assert.Equal(t, "market-1", mintTx.MarketID)
	assert.True(t, mintTx.Quantity.Equal(quantity))
	assert.True(t, mintTx.Cost.Equal(quantity), "cost should equal quantity for a complete set")

	// Assert: positions persisted in repo.
	positionRepo.mu.RLock()
	assert.Len(t, positionRepo.positions, 2, "2 positions should be persisted")
	positionRepo.mu.RUnlock()

	// Assert: all within same transaction.
	assert.True(t, txManager.txCalled, "minting should happen within a transaction")
}

// ---------------------------------------------------------------------------
// Tests: MintTokens -- Multi market creates N positions
// ---------------------------------------------------------------------------

func TestMintTokens_Multi_CreatesNPositions(t *testing.T) {
	svc, userRepo, outcomeRepo, positionRepo, tradeRepo, txManager := newTestMintService()
	ctx := context.Background()

	// Setup: user with balance 100, multi market with 3 outcomes (Red/Blue/Green).
	seedUser(userRepo, &domain.User{
		ID:            "user-2",
		WalletAddress: "0xdef",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})
	seedOutcomesForMint(outcomeRepo, []*domain.Outcome{
		{ID: "o-red", MarketID: "market-2", Label: "Red", Index: 0},
		{ID: "o-blue", MarketID: "market-2", Label: "Blue", Index: 1},
		{ID: "o-green", MarketID: "market-2", Label: "Green", Index: 2},
	})

	// Action: Mint 5 tokens.
	quantity := decimal.NewFromInt(5)
	positions, err := svc.MintTokens(ctx, "user-2", "market-2", quantity)
	require.NoError(t, err)

	// Assert: 3 positions created (Red qty=5, Blue qty=5, Green qty=5).
	require.Len(t, positions, 3, "multi mint with 3 outcomes should produce 3 positions")

	outcomeIDs := map[string]bool{}
	for _, p := range positions {
		assert.Equal(t, "user-2", p.UserID)
		assert.Equal(t, "market-2", p.MarketID)
		assert.True(t, p.Quantity.Equal(quantity),
			"each position quantity should equal minted amount, got: %s", p.Quantity)
		outcomeIDs[p.OutcomeID] = true
	}
	assert.True(t, outcomeIDs["o-red"], "should have a position for Red")
	assert.True(t, outcomeIDs["o-blue"], "should have a position for Blue")
	assert.True(t, outcomeIDs["o-green"], "should have a position for Green")

	// Assert: User balance deducted by 5 (cost = quantity for complete set).
	user, err := userRepo.GetByID(ctx, "user-2")
	require.NoError(t, err)
	assert.True(t, user.Balance.Equal(decimal.NewFromInt(95)),
		"user balance should be 95 after minting 5, got: %s", user.Balance)

	// Assert: MintTransaction recorded.
	tradeRepo.mu.RLock()
	require.Len(t, tradeRepo.mintTxs, 1, "one mint transaction should be recorded")
	assert.True(t, tradeRepo.mintTxs[0].Quantity.Equal(quantity))
	tradeRepo.mu.RUnlock()

	// Assert: positions persisted.
	positionRepo.mu.RLock()
	assert.Len(t, positionRepo.positions, 3, "3 positions should be persisted")
	positionRepo.mu.RUnlock()

	// Assert: transactional.
	assert.True(t, txManager.txCalled, "minting should happen within a transaction")
}

// ---------------------------------------------------------------------------
// Tests: MintTokens -- Balance deduction
// ---------------------------------------------------------------------------

func TestMintTokens_DeductsBalance(t *testing.T) {
	svc, userRepo, outcomeRepo, _, _, _ := newTestMintService()
	ctx := context.Background()

	// Setup: user with balance 50.
	seedUser(userRepo, &domain.User{
		ID:            "user-3",
		WalletAddress: "0x123",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(50),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})
	seedOutcomesForMint(outcomeRepo, []*domain.Outcome{
		{ID: "o-y", MarketID: "market-3", Label: "Yes", Index: 0},
		{ID: "o-n", MarketID: "market-3", Label: "No", Index: 1},
	})

	// Action: Mint 30 tokens on binary market.
	quantity := decimal.NewFromInt(30)
	_, err := svc.MintTokens(ctx, "user-3", "market-3", quantity)
	require.NoError(t, err)

	// Assert: user balance becomes 20 (50 - 30).
	user, err := userRepo.GetByID(ctx, "user-3")
	require.NoError(t, err)
	assert.True(t, user.Balance.Equal(decimal.NewFromInt(20)),
		"user balance should be 20 after minting 30 from 50, got: %s", user.Balance)
}

// ---------------------------------------------------------------------------
// Tests: MintTokens -- Insufficient balance
// ---------------------------------------------------------------------------

func TestMintTokens_InsufficientBalance_ReturnsError(t *testing.T) {
	svc, userRepo, outcomeRepo, positionRepo, tradeRepo, _ := newTestMintService()
	ctx := context.Background()

	// Setup: user with balance 5.
	seedUser(userRepo, &domain.User{
		ID:            "user-4",
		WalletAddress: "0x456",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(5),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})
	seedOutcomesForMint(outcomeRepo, []*domain.Outcome{
		{ID: "o-a", MarketID: "market-4", Label: "Yes", Index: 0},
		{ID: "o-b", MarketID: "market-4", Label: "No", Index: 1},
	})

	// Action: Try to mint 10 tokens (requires 10, only have 5).
	quantity := decimal.NewFromInt(10)
	positions, err := svc.MintTokens(ctx, "user-4", "market-4", quantity)

	// Assert: INSUFFICIENT_BALANCE error.
	assert.Error(t, err, "should return an error when balance is insufficient")
	assert.True(t, apperrors.IsInsufficientBalance(err),
		"error should be INSUFFICIENT_BALANCE, got: %v", err)
	assert.Nil(t, positions, "no positions should be returned on insufficient balance")

	// Assert: no positions created.
	positionRepo.mu.RLock()
	assert.Empty(t, positionRepo.positions, "no positions should be persisted")
	positionRepo.mu.RUnlock()

	// Assert: no balance change.
	user, err := userRepo.GetByID(ctx, "user-4")
	require.NoError(t, err)
	assert.True(t, user.Balance.Equal(decimal.NewFromInt(5)),
		"user balance should remain 5, got: %s", user.Balance)

	// Assert: no mint transaction.
	tradeRepo.mu.RLock()
	assert.Empty(t, tradeRepo.mintTxs, "no mint transaction should be recorded")
	tradeRepo.mu.RUnlock()
}

// ---------------------------------------------------------------------------
// Tests: MintTokens -- Transaction rollback on partial failure
// ---------------------------------------------------------------------------

func TestMintTokens_TxRollback_OnPartialFailure(t *testing.T) {
	svc, userRepo, outcomeRepo, positionRepo, _, txManager := newTestMintService()
	ctx := context.Background()

	// Setup: user with balance 100, but position upsert fails on second outcome.
	seedUser(userRepo, &domain.User{
		ID:            "user-5",
		WalletAddress: "0x789",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.Zero,
		CreatedAt:     time.Now(),
	})
	seedOutcomesForMint(outcomeRepo, []*domain.Outcome{
		{ID: "o-1", MarketID: "market-5", Label: "Yes", Index: 0},
		{ID: "o-2", MarketID: "market-5", Label: "No", Index: 1},
	})

	// Configure the position repo to fail on the second Upsert call.
	positionRepo.upsertErr = errors.New("simulated database failure")
	positionRepo.failOnCall = 2

	// Action: attempt to mint.
	quantity := decimal.NewFromInt(10)
	positions, err := svc.MintTokens(ctx, "user-5", "market-5", quantity)

	// Assert: error is returned (the transaction fn should propagate the upsert error).
	assert.Error(t, err, "should return an error when position upsert fails partway through")
	assert.Nil(t, positions, "no positions should be returned on failure")

	// Assert: TxManager was invoked (indicating the service attempted a transaction).
	assert.True(t, txManager.txCalled, "WithTx should have been called")

	// Note: In a real implementation with a proper TxManager, the database
	// transaction would roll back all changes. Our mock TxManager executes
	// the function directly, so partial state may exist in the mocks. The key
	// assertion is that the service correctly returns an error and the
	// transaction boundary was established, so a real DB would roll back.
}
