package service

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// ---------------------------------------------------------------------------
// Mock: TxManager
// ---------------------------------------------------------------------------

// mockTxManager executes the supplied function directly (no real transaction).
// txCalled tracks whether WithTx was invoked so tests can assert transactional
// behaviour.
type mockTxManager struct {
	txCalled bool
}

func (m *mockTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	m.txCalled = true
	return fn(ctx)
}

// ---------------------------------------------------------------------------
// Mock: MarketRepository
// ---------------------------------------------------------------------------

type mockMarketRepo struct {
	markets map[string]*domain.Market
	mu      sync.RWMutex
}

func newMockMarketRepo() *mockMarketRepo {
	return &mockMarketRepo{
		markets: make(map[string]*domain.Market),
	}
}

func (m *mockMarketRepo) Create(ctx context.Context, market *domain.Market) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markets[market.ID] = market
	return nil
}

func (m *mockMarketRepo) GetByID(ctx context.Context, id string) (*domain.Market, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mkt, ok := m.markets[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return mkt, nil
}

func (m *mockMarketRepo) Update(ctx context.Context, market *domain.Market) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.markets[market.ID]; !ok {
		return apperrors.ErrNotFound
	}
	m.markets[market.ID] = market
	return nil
}

func (m *mockMarketRepo) List(ctx context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []*domain.Market
	for _, mkt := range m.markets {
		if filter.Status != nil && mkt.Status != *filter.Status {
			continue
		}
		if filter.Category != nil && mkt.Category != *filter.Category {
			continue
		}
		filtered = append(filtered, mkt)
	}

	// Sort by CreatedAt descending for deterministic pagination.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := int64(len(filtered))

	// Apply pagination.
	if filter.Offset > 0 && filter.Offset < len(filtered) {
		filtered = filtered[filter.Offset:]
	} else if filter.Offset >= len(filtered) {
		filtered = nil
	}
	if filter.Limit > 0 && filter.Limit < len(filtered) {
		filtered = filtered[:filter.Limit]
	}

	return filtered, total, nil
}

// ---------------------------------------------------------------------------
// Mock: OutcomeRepository
// ---------------------------------------------------------------------------

type mockOutcomeRepo struct {
	outcomes map[string][]*domain.Outcome // marketID -> outcomes
	mu       sync.RWMutex
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
// Test helpers
// ---------------------------------------------------------------------------

// newTestMarketService creates a MarketService wired with in-memory mocks.
func newTestMarketService() (*MarketService, *mockMarketRepo, *mockOutcomeRepo, *mockTxManager) {
	marketRepo := newMockMarketRepo()
	outcomeRepo := newMockOutcomeRepo()
	txManager := &mockTxManager{}

	svc := NewMarketService(marketRepo, outcomeRepo, txManager)
	return svc, marketRepo, outcomeRepo, txManager
}

// seedMarket inserts a market directly into the mock repo so tests can
// reference an "existing" market without going through the service.
func seedMarket(repo *mockMarketRepo, market *domain.Market) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.markets[market.ID] = market
}

// seedOutcomes inserts outcomes directly into the mock repo.
func seedOutcomes(repo *mockOutcomeRepo, outcomes []*domain.Outcome) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, o := range outcomes {
		repo.outcomes[o.MarketID] = append(repo.outcomes[o.MarketID], o)
	}
}

// ptrStatus is a helper that returns a pointer to a MarketStatus value.
func ptrStatus(s domain.MarketStatus) *domain.MarketStatus { return &s }

// ptrString is a helper that returns a pointer to a string value.
func ptrString(s string) *string { return &s }

// ---------------------------------------------------------------------------
// Tests: CreateMarket -- Binary
// ---------------------------------------------------------------------------

func TestCreateMarket_Binary_CreatesYesNoOutcomes(t *testing.T) {
	svc, marketRepo, outcomeRepo, txManager := newTestMarketService()
	ctx := context.Background()

	req := CreateMarketRequest{
		Title:       "Will BTC exceed $100k by end of 2026?",
		Description: "Resolves YES if Bitcoin price exceeds $100,000 on any major exchange.",
		MarketType:  domain.MarketTypeBinary,
		Category:    "crypto",
		EndTime:     time.Now().Add(90 * 24 * time.Hour),
		CreatedBy:   "user-1",
	}

	market, err := svc.CreateMarket(ctx, req)
	require.NoError(t, err)

	// Market should be persisted.
	assert.NotEmpty(t, market.ID, "market ID should be assigned")
	assert.Equal(t, req.Title, market.Title)
	assert.Equal(t, req.Description, market.Description)
	assert.Equal(t, domain.MarketTypeBinary, market.MarketType)
	assert.Equal(t, domain.MarketStatusDraft, market.Status, "new market should start in draft")
	assert.Equal(t, req.Category, market.Category)
	assert.Equal(t, req.CreatedBy, market.CreatorID)

	// Verify market exists in the repository.
	persisted, err := marketRepo.GetByID(ctx, market.ID)
	require.NoError(t, err)
	assert.Equal(t, market.ID, persisted.ID)

	// Binary market must create exactly 2 outcomes: "Yes" and "No".
	outcomes, err := outcomeRepo.ListByMarket(ctx, market.ID)
	require.NoError(t, err)
	require.Len(t, outcomes, 2, "binary market should have exactly 2 outcomes")

	// Sort by index for deterministic assertions.
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Index < outcomes[j].Index
	})
	assert.Equal(t, "Yes", outcomes[0].Label)
	assert.Equal(t, 0, outcomes[0].Index)
	assert.Equal(t, "No", outcomes[1].Label)
	assert.Equal(t, 1, outcomes[1].Index)

	// Both outcomes should reference the correct market.
	for _, o := range outcomes {
		assert.Equal(t, market.ID, o.MarketID)
		assert.NotEmpty(t, o.ID, "outcome ID should be assigned")
		assert.False(t, o.IsWinner, "outcome should not be a winner at creation")
	}

	// Creation should happen within a transaction.
	assert.True(t, txManager.txCalled, "market and outcomes should be created within a transaction")
}

// ---------------------------------------------------------------------------
// Tests: CreateMarket -- Multi
// ---------------------------------------------------------------------------

func TestCreateMarket_Multi_CreatesAllOutcomes(t *testing.T) {
	svc, _, outcomeRepo, txManager := newTestMarketService()
	ctx := context.Background()

	labels := []string{"Red", "Blue", "Green"}
	req := CreateMarketRequest{
		Title:         "Which colour wins the 2026 championship?",
		Description:   "Resolves to the winning team colour.",
		MarketType:    domain.MarketTypeMulti,
		Category:      "sports",
		OutcomeLabels: labels,
		EndTime:       time.Now().Add(30 * 24 * time.Hour),
		CreatedBy:     "user-2",
	}

	market, err := svc.CreateMarket(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, domain.MarketTypeMulti, market.MarketType)

	// Multi market should create one outcome per label.
	outcomes, err := outcomeRepo.ListByMarket(ctx, market.ID)
	require.NoError(t, err)
	require.Len(t, outcomes, 3, "multi market should have one outcome per label")

	// Sort by index for deterministic checks.
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Index < outcomes[j].Index
	})

	for i, label := range labels {
		assert.Equal(t, label, outcomes[i].Label, "outcome label at index %d", i)
		assert.Equal(t, i, outcomes[i].Index, "outcome index")
		assert.Equal(t, market.ID, outcomes[i].MarketID)
	}

	assert.True(t, txManager.txCalled, "multi market creation should be transactional")
}

// ---------------------------------------------------------------------------
// Tests: CreateMarket -- Validation
// ---------------------------------------------------------------------------

func TestCreateMarket_EmptyTitle_ReturnsValidationError(t *testing.T) {
	svc, _, _, _ := newTestMarketService()
	ctx := context.Background()

	req := CreateMarketRequest{
		Title:       "",
		Description: "Some description",
		MarketType:  domain.MarketTypeBinary,
		Category:    "general",
		EndTime:     time.Now().Add(24 * time.Hour),
		CreatedBy:   "user-1",
	}

	market, err := svc.CreateMarket(ctx, req)

	assert.Error(t, err, "empty title should produce an error")
	assert.True(t, apperrors.IsBadRequest(err),
		"error should be BAD_REQUEST, got: %v", err)
	assert.Nil(t, market, "no market should be returned on validation error")
}

func TestCreateMarket_BinaryWithWrongOutcomeCount_ReturnsError(t *testing.T) {
	svc, _, _, _ := newTestMarketService()
	ctx := context.Background()

	// Binary market should NOT accept custom outcome labels (or exactly 2).
	// Passing 3 labels for a binary market is invalid.
	req := CreateMarketRequest{
		Title:         "Will it rain tomorrow?",
		Description:   "Simple yes/no question",
		MarketType:    domain.MarketTypeBinary,
		Category:      "weather",
		OutcomeLabels: []string{"Yes", "No", "Maybe"},
		EndTime:       time.Now().Add(24 * time.Hour),
		CreatedBy:     "user-1",
	}

	market, err := svc.CreateMarket(ctx, req)

	assert.Error(t, err, "binary market with 3 outcomes should produce an error")
	assert.True(t, apperrors.IsBadRequest(err),
		"error should be BAD_REQUEST, got: %v", err)
	assert.Nil(t, market, "no market should be returned when outcome count is wrong")
}

// ---------------------------------------------------------------------------
// Tests: ListMarkets
// ---------------------------------------------------------------------------

func TestListMarkets_ReturnsOpenMarkets(t *testing.T) {
	svc, marketRepo, _, _ := newTestMarketService()
	ctx := context.Background()

	// Seed 3 markets: 1 draft, 2 open.
	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID: "m-draft", Title: "Draft Market", Status: domain.MarketStatusDraft,
		MarketType: domain.MarketTypeBinary, Category: "general", CreatedAt: now,
	})
	seedMarket(marketRepo, &domain.Market{
		ID: "m-open-1", Title: "Open Market 1", Status: domain.MarketStatusOpen,
		MarketType: domain.MarketTypeBinary, Category: "crypto", CreatedAt: now.Add(1 * time.Second),
	})
	seedMarket(marketRepo, &domain.Market{
		ID: "m-open-2", Title: "Open Market 2", Status: domain.MarketStatusOpen,
		MarketType: domain.MarketTypeMulti, Category: "sports", CreatedAt: now.Add(2 * time.Second),
	})

	markets, total, err := svc.ListMarkets(ctx, repository.MarketFilter{
		Status: ptrStatus(domain.MarketStatusOpen),
	})
	require.NoError(t, err)

	assert.Equal(t, int64(2), total, "total should be 2 open markets")
	assert.Len(t, markets, 2, "should return 2 open markets")
	for _, m := range markets {
		assert.Equal(t, domain.MarketStatusOpen, m.Status, "all returned markets should be open")
	}
}

func TestListMarkets_FilterByCategory(t *testing.T) {
	svc, marketRepo, _, _ := newTestMarketService()
	ctx := context.Background()

	now := time.Now()
	seedMarket(marketRepo, &domain.Market{
		ID: "m-crypto", Title: "BTC Market", Status: domain.MarketStatusOpen,
		MarketType: domain.MarketTypeBinary, Category: "crypto", CreatedAt: now,
	})
	seedMarket(marketRepo, &domain.Market{
		ID: "m-sports", Title: "Football Market", Status: domain.MarketStatusOpen,
		MarketType: domain.MarketTypeBinary, Category: "sports", CreatedAt: now.Add(1 * time.Second),
	})
	seedMarket(marketRepo, &domain.Market{
		ID: "m-crypto-2", Title: "ETH Market", Status: domain.MarketStatusOpen,
		MarketType: domain.MarketTypeBinary, Category: "crypto", CreatedAt: now.Add(2 * time.Second),
	})

	markets, total, err := svc.ListMarkets(ctx, repository.MarketFilter{
		Category: ptrString("crypto"),
	})
	require.NoError(t, err)

	assert.Equal(t, int64(2), total, "total should be 2 crypto markets")
	assert.Len(t, markets, 2)
	for _, m := range markets {
		assert.Equal(t, "crypto", m.Category, "all returned markets should be in crypto category")
	}
}

func TestListMarkets_Pagination(t *testing.T) {
	svc, marketRepo, _, _ := newTestMarketService()
	ctx := context.Background()

	// Seed 5 open markets.
	now := time.Now()
	for i := 0; i < 5; i++ {
		seedMarket(marketRepo, &domain.Market{
			ID:         "m-page-" + string(rune('A'+i)),
			Title:      "Market " + string(rune('A'+i)),
			Status:     domain.MarketStatusOpen,
			MarketType: domain.MarketTypeBinary,
			Category:   "general",
			CreatedAt:  now.Add(time.Duration(i) * time.Second),
		})
	}

	// Request page 1: limit=2, offset=0.
	markets, total, err := svc.ListMarkets(ctx, repository.MarketFilter{
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)

	assert.Equal(t, int64(5), total, "total should be 5 regardless of pagination")
	assert.Len(t, markets, 2, "page 1 should return 2 items")

	// Request page 2: limit=2, offset=2.
	markets2, total2, err := svc.ListMarkets(ctx, repository.MarketFilter{
		Limit:  2,
		Offset: 2,
	})
	require.NoError(t, err)

	assert.Equal(t, int64(5), total2, "total should still be 5")
	assert.Len(t, markets2, 2, "page 2 should return 2 items")

	// Request page 3: limit=2, offset=4 -- only 1 item left.
	markets3, _, err := svc.ListMarkets(ctx, repository.MarketFilter{
		Limit:  2,
		Offset: 4,
	})
	require.NoError(t, err)
	assert.Len(t, markets3, 1, "page 3 should return 1 item (remainder)")
}

// ---------------------------------------------------------------------------
// Tests: GetMarket
// ---------------------------------------------------------------------------

func TestGetMarket_ReturnsWithOutcomes(t *testing.T) {
	svc, marketRepo, outcomeRepo, _ := newTestMarketService()
	ctx := context.Background()

	// Seed a market and its outcomes.
	seedMarket(marketRepo, &domain.Market{
		ID:         "m-get-1",
		Title:      "Test Market",
		Status:     domain.MarketStatusOpen,
		MarketType: domain.MarketTypeBinary,
		Category:   "general",
		CreatorID:  "user-1",
		CreatedAt:  time.Now(),
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-1", MarketID: "m-get-1", Label: "Yes", Index: 0},
		{ID: "o-2", MarketID: "m-get-1", Label: "No", Index: 1},
	})

	market, outcomes, err := svc.GetMarket(ctx, "m-get-1")
	require.NoError(t, err)

	assert.Equal(t, "m-get-1", market.ID)
	assert.Equal(t, "Test Market", market.Title)
	require.Len(t, outcomes, 2, "should return outcomes for the market")
	assert.Equal(t, "Yes", outcomes[0].Label)
	assert.Equal(t, "No", outcomes[1].Label)
}

func TestGetMarket_NotFound_ReturnsError(t *testing.T) {
	svc, _, _, _ := newTestMarketService()
	ctx := context.Background()

	market, outcomes, err := svc.GetMarket(ctx, "nonexistent-market-id")

	assert.Error(t, err, "should return an error for missing market")
	assert.True(t, apperrors.IsNotFound(err),
		"error should be NOT_FOUND, got: %v", err)
	assert.Nil(t, market, "no market should be returned when not found")
	assert.Nil(t, outcomes, "no outcomes should be returned when market not found")
}

// ---------------------------------------------------------------------------
// Tests: UpdateMarketStatus
// ---------------------------------------------------------------------------

func TestUpdateMarketStatus_ValidTransitions(t *testing.T) {
	svc, marketRepo, _, _ := newTestMarketService()
	ctx := context.Background()

	// Test draft -> open.
	seedMarket(marketRepo, &domain.Market{
		ID: "m-trans-1", Title: "Transition Test 1", Status: domain.MarketStatusDraft,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	err := svc.UpdateMarketStatus(ctx, "m-trans-1", domain.MarketStatusOpen)
	require.NoError(t, err, "draft -> open should be valid")

	updated, err := marketRepo.GetByID(ctx, "m-trans-1")
	require.NoError(t, err)
	assert.Equal(t, domain.MarketStatusOpen, updated.Status, "market should now be open")

	// Test open -> closed.
	err = svc.UpdateMarketStatus(ctx, "m-trans-1", domain.MarketStatusClosed)
	require.NoError(t, err, "open -> closed should be valid")

	updated, err = marketRepo.GetByID(ctx, "m-trans-1")
	require.NoError(t, err)
	assert.Equal(t, domain.MarketStatusClosed, updated.Status, "market should now be closed")

	// Test closed -> resolved.
	seedMarket(marketRepo, &domain.Market{
		ID: "m-trans-2", Title: "Transition Test 2", Status: domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	err = svc.UpdateMarketStatus(ctx, "m-trans-2", domain.MarketStatusResolved)
	require.NoError(t, err, "closed -> resolved should be valid")

	updated, err = marketRepo.GetByID(ctx, "m-trans-2")
	require.NoError(t, err)
	assert.Equal(t, domain.MarketStatusResolved, updated.Status, "market should now be resolved")

	// Test closed -> cancelled.
	seedMarket(marketRepo, &domain.Market{
		ID: "m-trans-3", Title: "Transition Test 3", Status: domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	err = svc.UpdateMarketStatus(ctx, "m-trans-3", domain.MarketStatusCancelled)
	require.NoError(t, err, "closed -> cancelled should be valid")

	updated, err = marketRepo.GetByID(ctx, "m-trans-3")
	require.NoError(t, err)
	assert.Equal(t, domain.MarketStatusCancelled, updated.Status, "market should now be cancelled")
}

func TestUpdateMarketStatus_InvalidTransition_ReturnsError(t *testing.T) {
	svc, marketRepo, _, _ := newTestMarketService()
	ctx := context.Background()

	// draft -> resolved is invalid (must go through open -> closed first).
	seedMarket(marketRepo, &domain.Market{
		ID: "m-invalid-1", Title: "Invalid Transition", Status: domain.MarketStatusDraft,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	err := svc.UpdateMarketStatus(ctx, "m-invalid-1", domain.MarketStatusResolved)
	assert.Error(t, err, "draft -> resolved should fail")
	assert.True(t, apperrors.IsBadRequest(err),
		"error should be BAD_REQUEST for invalid transition, got: %v", err)

	// Verify status was NOT changed.
	unchanged, getErr := marketRepo.GetByID(ctx, "m-invalid-1")
	require.NoError(t, getErr)
	assert.Equal(t, domain.MarketStatusDraft, unchanged.Status,
		"market status should remain draft after invalid transition")

	// open -> draft is invalid (can't go backwards).
	seedMarket(marketRepo, &domain.Market{
		ID: "m-invalid-2", Title: "Backwards Transition", Status: domain.MarketStatusOpen,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	err = svc.UpdateMarketStatus(ctx, "m-invalid-2", domain.MarketStatusDraft)
	assert.Error(t, err, "open -> draft should fail")
	assert.True(t, apperrors.IsBadRequest(err),
		"error should be BAD_REQUEST for backwards transition, got: %v", err)

	// resolved -> open is invalid (terminal state).
	seedMarket(marketRepo, &domain.Market{
		ID: "m-invalid-3", Title: "Terminal State", Status: domain.MarketStatusResolved,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	err = svc.UpdateMarketStatus(ctx, "m-invalid-3", domain.MarketStatusOpen)
	assert.Error(t, err, "resolved -> open should fail")
	assert.True(t, apperrors.IsBadRequest(err),
		"error should be BAD_REQUEST for transition from terminal state, got: %v", err)
}

func TestUpdateMarketStatus_NonExistentMarket_ReturnsError(t *testing.T) {
	svc, _, _, _ := newTestMarketService()
	ctx := context.Background()

	err := svc.UpdateMarketStatus(ctx, "nonexistent-id", domain.MarketStatusOpen)

	assert.Error(t, err, "should return error for nonexistent market")
	assert.True(t, apperrors.IsNotFound(err),
		"error should be NOT_FOUND, got: %v", err)
}

// ---------------------------------------------------------------------------
// Tests: ResolveMarket
// ---------------------------------------------------------------------------

func TestResolveMarket_SetsWinningOutcome(t *testing.T) {
	svc, marketRepo, outcomeRepo, txManager := newTestMarketService()
	ctx := context.Background()

	// Seed a closed market with outcomes ready for resolution.
	seedMarket(marketRepo, &domain.Market{
		ID: "m-resolve", Title: "Resolution Market", Status: domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-yes", MarketID: "m-resolve", Label: "Yes", Index: 0},
		{ID: "o-no", MarketID: "m-resolve", Label: "No", Index: 1},
	})

	// Reset txCalled so we can verify resolution uses a transaction.
	txManager.txCalled = false

	err := svc.ResolveMarket(ctx, "m-resolve", "o-yes")
	require.NoError(t, err)

	// Market status should now be resolved.
	resolved, err := marketRepo.GetByID(ctx, "m-resolve")
	require.NoError(t, err)
	assert.Equal(t, domain.MarketStatusResolved, resolved.Status, "market should be resolved")
	require.NotNil(t, resolved.ResolvedOutcomeID, "resolved outcome ID should be set")
	assert.Equal(t, "o-yes", *resolved.ResolvedOutcomeID)

	// The winning outcome should be flagged.
	outcomes, err := outcomeRepo.ListByMarket(ctx, "m-resolve")
	require.NoError(t, err)
	for _, o := range outcomes {
		if o.ID == "o-yes" {
			assert.True(t, o.IsWinner, "winning outcome should have IsWinner=true")
		} else {
			assert.False(t, o.IsWinner, "non-winning outcome should have IsWinner=false")
		}
	}

	// Resolution should be transactional.
	assert.True(t, txManager.txCalled, "resolution should happen within a transaction")
}

func TestResolveMarket_NotClosed_ReturnsError(t *testing.T) {
	svc, marketRepo, outcomeRepo, _ := newTestMarketService()
	ctx := context.Background()

	// Seed a market that is still open (not closed).
	seedMarket(marketRepo, &domain.Market{
		ID: "m-not-closed", Title: "Still Open", Status: domain.MarketStatusOpen,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-a", MarketID: "m-not-closed", Label: "Yes", Index: 0},
		{ID: "o-b", MarketID: "m-not-closed", Label: "No", Index: 1},
	})

	err := svc.ResolveMarket(ctx, "m-not-closed", "o-a")

	assert.Error(t, err, "resolving a non-closed market should fail")
	assert.True(t, apperrors.IsBadRequest(err),
		"error should be BAD_REQUEST, got: %v", err)

	// Status should not have changed.
	m, _ := marketRepo.GetByID(ctx, "m-not-closed")
	assert.Equal(t, domain.MarketStatusOpen, m.Status, "status should remain open")
}

func TestResolveMarket_InvalidOutcomeID_ReturnsError(t *testing.T) {
	svc, marketRepo, outcomeRepo, _ := newTestMarketService()
	ctx := context.Background()

	seedMarket(marketRepo, &domain.Market{
		ID: "m-bad-outcome", Title: "Bad Outcome", Status: domain.MarketStatusClosed,
		MarketType: domain.MarketTypeBinary, Category: "general",
		CreatorID: "user-1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	seedOutcomes(outcomeRepo, []*domain.Outcome{
		{ID: "o-x", MarketID: "m-bad-outcome", Label: "Yes", Index: 0},
		{ID: "o-y", MarketID: "m-bad-outcome", Label: "No", Index: 1},
	})

	// Try to resolve with an outcome ID that doesn't belong to this market.
	err := svc.ResolveMarket(ctx, "m-bad-outcome", "o-nonexistent")

	assert.Error(t, err, "resolving with invalid outcome ID should fail")
	assert.True(t, apperrors.IsBadRequest(err) || apperrors.IsNotFound(err),
		"error should be BAD_REQUEST or NOT_FOUND, got: %v", err)
}
