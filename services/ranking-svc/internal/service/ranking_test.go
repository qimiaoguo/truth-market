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
	"github.com/truthmarket/truth-market/pkg/repository"
)

// ---------------------------------------------------------------------------
// Mock: RankingRepository
// ---------------------------------------------------------------------------

type mockRankingRepo struct {
	mu           sync.RWMutex
	rankings     []*domain.UserRanking
	refreshCalled bool
	refreshErr   error
}

func (m *mockRankingRepo) Upsert(ctx context.Context, ranking *domain.UserRanking) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Update existing or append.
	for i, r := range m.rankings {
		if r.UserID == ranking.UserID && r.Dimension == ranking.Dimension {
			m.rankings[i] = ranking
			return nil
		}
	}
	m.rankings = append(m.rankings, ranking)
	return nil
}

func (m *mockRankingRepo) GetByUser(ctx context.Context, userID string) ([]*domain.UserRanking, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.UserRanking
	for _, r := range m.rankings {
		if r.UserID == userID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockRankingRepo) List(ctx context.Context, filter repository.RankingFilter) ([]*domain.UserRanking, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Apply filters.
	var filtered []*domain.UserRanking
	for _, r := range m.rankings {
		if filter.Dimension != nil && r.Dimension != *filter.Dimension {
			continue
		}
		if filter.UserType != nil && r.UserType != *filter.UserType {
			continue
		}
		filtered = append(filtered, r)
	}

	total := int64(len(filtered))

	// Apply pagination.
	start := filter.Offset
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + filter.Limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], total, nil
}

func (m *mockRankingRepo) RefreshMaterializedView(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshCalled = true
	return m.refreshErr
}

// ---------------------------------------------------------------------------
// Mock: UserRepository (for ranking service)
// ---------------------------------------------------------------------------

type mockRankingUserRepo struct {
	mu    sync.RWMutex
	users map[string]*domain.User
}

func (m *mockRankingUserRepo) Create(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
	return nil
}

func (m *mockRankingUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockRankingUserRepo) GetByWallet(ctx context.Context, addr string) (*domain.User, error) {
	return nil, nil
}

func (m *mockRankingUserRepo) UpdateBalance(ctx context.Context, id string, balance, locked decimal.Decimal) error {
	return nil
}

func (m *mockRankingUserRepo) List(ctx context.Context, filter repository.UserFilter) ([]*domain.User, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.User
	for _, u := range m.users {
		result = append(result, u)
	}
	return result, int64(len(result)), nil
}

// ---------------------------------------------------------------------------
// Test helper
// ---------------------------------------------------------------------------

func newTestRankingService() (*RankingService, *mockRankingRepo, *mockRankingUserRepo) {
	rankingRepo := &mockRankingRepo{rankings: []*domain.UserRanking{}}
	userRepo := &mockRankingUserRepo{users: make(map[string]*domain.User)}
	svc := NewRankingService(rankingRepo, userRepo)
	return svc, rankingRepo, userRepo
}

// ---------------------------------------------------------------------------
// Tests: GetLeaderboard -- Filter by dimension
// ---------------------------------------------------------------------------

func TestGetLeaderboard_ByDimension(t *testing.T) {
	svc, rankingRepo, _ := newTestRankingService()
	ctx := context.Background()

	now := time.Now()

	// Seed rankings across two dimensions: total_assets and pnl.
	rankingRepo.mu.Lock()
	rankingRepo.rankings = []*domain.UserRanking{
		{UserID: "user-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionTotalAssets, Value: decimal.NewFromInt(5000), Rank: 1, UpdatedAt: now},
		{UserID: "user-2", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionTotalAssets, Value: decimal.NewFromInt(3000), Rank: 2, UpdatedAt: now},
		{UserID: "user-3", UserType: domain.UserTypeAgent, Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(1200), Rank: 1, UpdatedAt: now},
		{UserID: "user-4", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(800), Rank: 2, UpdatedAt: now},
	}
	rankingRepo.mu.Unlock()

	// Action: Get leaderboard for total_assets dimension.
	rankings, total, err := svc.GetLeaderboard(ctx, domain.RankDimensionTotalAssets, nil, 1, 10)
	require.NoError(t, err)

	// Assert: only total_assets rankings returned.
	assert.Equal(t, int64(2), total, "total count should be 2 for total_assets dimension")
	require.Len(t, rankings, 2, "should return 2 rankings for total_assets")

	for _, r := range rankings {
		assert.Equal(t, domain.RankDimensionTotalAssets, r.Dimension,
			"all returned rankings should be total_assets dimension")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetLeaderboard -- Filter by user type
// ---------------------------------------------------------------------------

func TestGetLeaderboard_FilterByUserType(t *testing.T) {
	svc, rankingRepo, _ := newTestRankingService()
	ctx := context.Background()

	now := time.Now()

	// Seed pnl rankings for human and agent users.
	rankingRepo.mu.Lock()
	rankingRepo.rankings = []*domain.UserRanking{
		{UserID: "human-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(900), Rank: 1, UpdatedAt: now},
		{UserID: "agent-1", UserType: domain.UserTypeAgent, Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(1500), Rank: 2, UpdatedAt: now},
		{UserID: "human-2", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(400), Rank: 3, UpdatedAt: now},
		{UserID: "agent-2", UserType: domain.UserTypeAgent, Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(300), Rank: 4, UpdatedAt: now},
	}
	rankingRepo.mu.Unlock()

	// Action: Get leaderboard filtered to human users only.
	humanType := domain.UserTypeHuman
	rankings, total, err := svc.GetLeaderboard(ctx, domain.RankDimensionPnL, &humanType, 1, 10)
	require.NoError(t, err)

	// Assert: only human user rankings returned.
	assert.Equal(t, int64(2), total, "total count should be 2 for human users in pnl dimension")
	require.Len(t, rankings, 2, "should return 2 human rankings")

	for _, r := range rankings {
		assert.Equal(t, domain.UserTypeHuman, r.UserType,
			"all returned rankings should belong to human users")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetLeaderboard -- Pagination
// ---------------------------------------------------------------------------

func TestGetLeaderboard_Pagination(t *testing.T) {
	svc, rankingRepo, _ := newTestRankingService()
	ctx := context.Background()

	now := time.Now()

	// Seed 5 volume rankings.
	rankingRepo.mu.Lock()
	rankingRepo.rankings = []*domain.UserRanking{
		{UserID: "user-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionVolume, Value: decimal.NewFromInt(10000), Rank: 1, UpdatedAt: now},
		{UserID: "user-2", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionVolume, Value: decimal.NewFromInt(8000), Rank: 2, UpdatedAt: now},
		{UserID: "user-3", UserType: domain.UserTypeAgent, Dimension: domain.RankDimensionVolume, Value: decimal.NewFromInt(6000), Rank: 3, UpdatedAt: now},
		{UserID: "user-4", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionVolume, Value: decimal.NewFromInt(4000), Rank: 4, UpdatedAt: now},
		{UserID: "user-5", UserType: domain.UserTypeAgent, Dimension: domain.RankDimensionVolume, Value: decimal.NewFromInt(2000), Rank: 5, UpdatedAt: now},
	}
	rankingRepo.mu.Unlock()

	// Action: Request page 2 with 2 items per page.
	rankings, total, err := svc.GetLeaderboard(ctx, domain.RankDimensionVolume, nil, 2, 2)
	require.NoError(t, err)

	// Assert: total is 5 (all volume rankings), but only 2 returned for page 2.
	assert.Equal(t, int64(5), total, "total count should reflect all volume rankings")
	require.Len(t, rankings, 2, "page 2 with perPage=2 should return 2 items")
}

// ---------------------------------------------------------------------------
// Tests: GetUserRanking -- Returns all dimensions
// ---------------------------------------------------------------------------

func TestGetUserRanking_AllDimensions(t *testing.T) {
	svc, rankingRepo, _ := newTestRankingService()
	ctx := context.Background()

	now := time.Now()

	// Seed rankings for "user-1" across all 5 dimensions.
	rankingRepo.mu.Lock()
	rankingRepo.rankings = []*domain.UserRanking{
		{UserID: "user-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionTotalAssets, Value: decimal.NewFromInt(5000), Rank: 1, UpdatedAt: now},
		{UserID: "user-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(1200), Rank: 3, UpdatedAt: now},
		{UserID: "user-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionVolume, Value: decimal.NewFromInt(8000), Rank: 2, UpdatedAt: now},
		{UserID: "user-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionWinRate, Value: decimal.NewFromFloat(0.75), Rank: 5, UpdatedAt: now},
		{UserID: "user-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionTradeCount, Value: decimal.NewFromInt(42), Rank: 10, UpdatedAt: now},
		// Another user's ranking -- should not appear in result.
		{UserID: "user-2", UserType: domain.UserTypeAgent, Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(900), Rank: 4, UpdatedAt: now},
	}
	rankingRepo.mu.Unlock()

	// Action: Get all rankings for user-1.
	rankings, err := svc.GetUserRanking(ctx, "user-1")
	require.NoError(t, err)

	// Assert: exactly 5 rankings returned, one per dimension.
	require.Len(t, rankings, 5, "user-1 should have rankings across all 5 dimensions")

	dimensions := make(map[domain.RankDimension]bool)
	for _, r := range rankings {
		assert.Equal(t, "user-1", r.UserID, "all rankings should belong to user-1")
		dimensions[r.Dimension] = true
	}

	assert.True(t, dimensions[domain.RankDimensionTotalAssets], "should have total_assets ranking")
	assert.True(t, dimensions[domain.RankDimensionPnL], "should have pnl ranking")
	assert.True(t, dimensions[domain.RankDimensionVolume], "should have volume ranking")
	assert.True(t, dimensions[domain.RankDimensionWinRate], "should have win_rate ranking")
	assert.True(t, dimensions[domain.RankDimensionTradeCount], "should have trade_count ranking")
}

// ---------------------------------------------------------------------------
// Tests: RefreshRankings -- Calls repository
// ---------------------------------------------------------------------------

func TestRefreshRankings_CallsRepository(t *testing.T) {
	svc, rankingRepo, _ := newTestRankingService()
	ctx := context.Background()

	// Pre-condition: refresh has not been called.
	rankingRepo.mu.RLock()
	assert.False(t, rankingRepo.refreshCalled, "refreshCalled should be false before calling RefreshRankings")
	rankingRepo.mu.RUnlock()

	// Action: Refresh rankings.
	err := svc.RefreshRankings(ctx)
	require.NoError(t, err)

	// Assert: repository's RefreshMaterializedView was invoked.
	rankingRepo.mu.RLock()
	assert.True(t, rankingRepo.refreshCalled, "RefreshMaterializedView should have been called")
	rankingRepo.mu.RUnlock()
}
