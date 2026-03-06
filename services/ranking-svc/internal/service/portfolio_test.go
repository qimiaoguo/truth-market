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
// Mock: PositionRepository (for portfolio service)
// ---------------------------------------------------------------------------

type mockPortfolioPositionRepo struct {
	mu        sync.RWMutex
	positions []*domain.Position
}

func (m *mockPortfolioPositionRepo) Upsert(ctx context.Context, position *domain.Position) error {
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

func (m *mockPortfolioPositionRepo) GetByUserAndOutcome(ctx context.Context, userID, outcomeID string) (*domain.Position, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.positions {
		if p.UserID == userID && p.OutcomeID == outcomeID {
			return p, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *mockPortfolioPositionRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Position, error) {
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

func (m *mockPortfolioPositionRepo) ListByMarket(ctx context.Context, marketID string) ([]*domain.Position, error) {
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
// Test helper
// ---------------------------------------------------------------------------

func newTestPortfolioService() (*PortfolioService, *mockPortfolioPositionRepo, *mockRankingUserRepo) {
	positionRepo := &mockPortfolioPositionRepo{}
	userRepo := &mockRankingUserRepo{users: make(map[string]*domain.User)}
	svc := NewPortfolioService(positionRepo, userRepo)
	return svc, positionRepo, userRepo
}

// ---------------------------------------------------------------------------
// Tests: GetPortfolio -- Aggregates positions into total value
// ---------------------------------------------------------------------------

func TestGetPortfolio_AggregatesPositions(t *testing.T) {
	svc, positionRepo, userRepo := newTestPortfolioService()
	ctx := context.Background()

	now := time.Now()

	// Seed user with balance=500.
	userRepo.mu.Lock()
	userRepo.users["user-1"] = &domain.User{
		ID:            "user-1",
		WalletAddress: "0xabc",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(500),
		LockedBalance: decimal.Zero,
		CreatedAt:     now,
	}
	userRepo.mu.Unlock()

	// Seed 3 positions:
	// Position 1: qty=10, avg=0.6  -> value = 10 * 0.6 = 6
	// Position 2: qty=5,  avg=0.4  -> value = 5  * 0.4 = 2
	// Position 3: qty=20, avg=0.3  -> value = 20 * 0.3 = 6
	positionRepo.mu.Lock()
	positionRepo.positions = []*domain.Position{
		{ID: "pos-1", UserID: "user-1", MarketID: "market-1", OutcomeID: "out-1", Quantity: decimal.NewFromInt(10), AvgPrice: decimal.NewFromFloat(0.6), UpdatedAt: now},
		{ID: "pos-2", UserID: "user-1", MarketID: "market-2", OutcomeID: "out-2", Quantity: decimal.NewFromInt(5), AvgPrice: decimal.NewFromFloat(0.4), UpdatedAt: now},
		{ID: "pos-3", UserID: "user-1", MarketID: "market-3", OutcomeID: "out-3", Quantity: decimal.NewFromInt(20), AvgPrice: decimal.NewFromFloat(0.3), UpdatedAt: now},
	}
	positionRepo.mu.Unlock()

	// Action: Get portfolio for user-1.
	portfolio, err := svc.GetPortfolio(ctx, "user-1")
	require.NoError(t, err)

	// Assert: TotalValue = balance(500) + 6 + 2 + 6 = 514.
	expectedTotal := decimal.NewFromInt(514)
	assert.True(t, portfolio.TotalValue.Equal(expectedTotal),
		"TotalValue should be 514 (500 balance + 14 position value), got: %s", portfolio.TotalValue)

	// Assert: 3 positions in portfolio.
	require.Len(t, portfolio.Positions, 3, "portfolio should contain 3 positions")

	// Assert each position has correct Value = quantity * avgPrice.
	for _, pp := range portfolio.Positions {
		expectedValue := pp.Quantity.Mul(pp.AvgPrice)
		assert.True(t, pp.Value.Equal(expectedValue),
			"position %s value should be %s (qty=%s * avg=%s), got: %s",
			pp.OutcomeID, expectedValue, pp.Quantity, pp.AvgPrice, pp.Value)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetPortfolio -- Calculates unrealized PnL
// ---------------------------------------------------------------------------

func TestGetPortfolio_CalculatesUnrealizedPnL(t *testing.T) {
	svc, positionRepo, userRepo := newTestPortfolioService()
	ctx := context.Background()

	now := time.Now()

	// Seed user with balance=700 (started with 1000, spent 300 minting).
	userRepo.mu.Lock()
	userRepo.users["user-1"] = &domain.User{
		ID:            "user-1",
		WalletAddress: "0xdef",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(700),
		LockedBalance: decimal.Zero,
		CreatedAt:     now,
	}
	userRepo.mu.Unlock()

	// Seed 2 positions:
	// Position 1: qty=10, avg=0.5 -> value = 10 * 0.5 = 5
	// Position 2: qty=5,  avg=0.6 -> value = 5  * 0.6 = 3
	positionRepo.mu.Lock()
	positionRepo.positions = []*domain.Position{
		{ID: "pos-1", UserID: "user-1", MarketID: "market-1", OutcomeID: "out-1", Quantity: decimal.NewFromInt(10), AvgPrice: decimal.NewFromFloat(0.5), UpdatedAt: now},
		{ID: "pos-2", UserID: "user-1", MarketID: "market-2", OutcomeID: "out-2", Quantity: decimal.NewFromInt(5), AvgPrice: decimal.NewFromFloat(0.6), UpdatedAt: now},
	}
	positionRepo.mu.Unlock()

	// Action: Get portfolio for user-1.
	portfolio, err := svc.GetPortfolio(ctx, "user-1")
	require.NoError(t, err)

	// Assert: TotalValue = balance(700) + 5 + 3 = 708.
	expectedTotal := decimal.NewFromInt(708)
	assert.True(t, portfolio.TotalValue.Equal(expectedTotal),
		"TotalValue should be 708 (700 balance + 8 position value), got: %s", portfolio.TotalValue)

	// Assert: UnrealizedPnL = 708 - 1000 = -292 (user is down because tokens depreciated).
	expectedPnL := decimal.NewFromInt(-292)
	assert.True(t, portfolio.UnrealizedPnL.Equal(expectedPnL),
		"UnrealizedPnL should be -292 (708 total - 1000 initial), got: %s", portfolio.UnrealizedPnL)
}
