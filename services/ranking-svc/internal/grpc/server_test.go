package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/truthmarket/truth-market/pkg/domain"
	rankingv1 "github.com/truthmarket/truth-market/proto/gen/go/ranking/v1"
	rankinggrpc "github.com/truthmarket/truth-market/services/ranking-svc/internal/grpc"
)

// ---------------------------------------------------------------------------
// Mock services
// ---------------------------------------------------------------------------

// mockRankingServicer implements rankinggrpc.RankingServicer for testing.
type mockRankingServicer struct {
	getLeaderboardFn  func(ctx context.Context, dimension domain.RankDimension, userType *domain.UserType, page, perPage int) ([]*domain.UserRanking, int64, error)
	getUserRankingFn  func(ctx context.Context, userID string) ([]*domain.UserRanking, error)
	refreshRankingsFn func(ctx context.Context) error
}

func (m *mockRankingServicer) GetLeaderboard(ctx context.Context, dimension domain.RankDimension, userType *domain.UserType, page, perPage int) ([]*domain.UserRanking, int64, error) {
	return m.getLeaderboardFn(ctx, dimension, userType, page, perPage)
}

func (m *mockRankingServicer) GetUserRanking(ctx context.Context, userID string) ([]*domain.UserRanking, error) {
	return m.getUserRankingFn(ctx, userID)
}

func (m *mockRankingServicer) RefreshRankings(ctx context.Context) error {
	return m.refreshRankingsFn(ctx)
}

// mockPortfolioServicer implements rankinggrpc.PortfolioServicer for testing.
type mockPortfolioServicer struct {
	getPortfolioFn func(ctx context.Context, userID string) (*rankinggrpc.Portfolio, error)
}

func (m *mockPortfolioServicer) GetPortfolio(ctx context.Context, userID string) (*rankinggrpc.Portfolio, error) {
	return m.getPortfolioFn(ctx, userID)
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

const bufSize = 1024 * 1024

// testEnv bundles a bufconn-based gRPC client together with the mock services
// so each test can configure behaviour and make real gRPC calls.
type testEnv struct {
	client       rankingv1.RankingServiceClient
	rankingSvc   *mockRankingServicer
	portfolioSvc *mockPortfolioServicer
	conn         *grpc.ClientConn
	grpcServer   *grpc.Server
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	rankingSvc := &mockRankingServicer{}
	portfolioSvc := &mockPortfolioServicer{}

	srv := rankinggrpc.NewRankingServer(rankingSvc, portfolioSvc)

	lis := bufconn.Listen(bufSize)
	gs := grpc.NewServer()
	rankingv1.RegisterRankingServiceServer(gs, srv)

	go func() {
		if err := gs.Serve(lis); err != nil {
			// The server will return an error when we call GracefulStop; that
			// is expected and does not indicate a real failure.
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		gs.GracefulStop()
	})

	return &testEnv{
		client:       rankingv1.NewRankingServiceClient(conn),
		rankingSvc:   rankingSvc,
		portfolioSvc: portfolioSvc,
		conn:         conn,
		grpcServer:   gs,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGRPC_GetLeaderboard_ReturnsPaginated(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	rankings := []*domain.UserRanking{
		{UserID: "user-1", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionTotalAssets, Value: decimal.NewFromInt(5000), Rank: 1, UpdatedAt: now},
		{UserID: "user-2", UserType: domain.UserTypeHuman, Dimension: domain.RankDimensionTotalAssets, Value: decimal.NewFromInt(4000), Rank: 2, UpdatedAt: now},
		{UserID: "user-3", UserType: domain.UserTypeAgent, Dimension: domain.RankDimensionTotalAssets, Value: decimal.NewFromInt(3000), Rank: 3, UpdatedAt: now},
	}

	env.rankingSvc.getLeaderboardFn = func(_ context.Context, dimension domain.RankDimension, _ *domain.UserType, page, perPage int) ([]*domain.UserRanking, int64, error) {
		assert.Equal(t, domain.RankDimensionTotalAssets, dimension)
		assert.Equal(t, 1, page)
		assert.Equal(t, 3, perPage)
		return rankings, int64(10), nil
	}

	resp, err := env.client.GetLeaderboard(context.Background(), &rankingv1.GetLeaderboardRequest{
		Dimension: rankingv1.RankDimension_RANK_DIMENSION_TOTAL_ASSETS,
		Page:      1,
		PerPage:   3,
	})
	require.NoError(t, err)

	require.Len(t, resp.GetRankings(), 3, "should return 3 rankings")
	assert.Equal(t, int64(10), resp.GetTotal(), "total should reflect all matching rankings")

	first := resp.GetRankings()[0]
	assert.Equal(t, "user-1", first.GetUserId())
	assert.Equal(t, int64(1), first.GetRank())
	assert.Equal(t, "5000", first.GetValue())
}

func TestGRPC_GetUserRanking_ReturnsAllDimensions(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	rankings := []*domain.UserRanking{
		{UserID: "user-1", Dimension: domain.RankDimensionTotalAssets, Value: decimal.NewFromInt(5000), Rank: 1, UpdatedAt: now},
		{UserID: "user-1", Dimension: domain.RankDimensionPnL, Value: decimal.NewFromInt(1200), Rank: 3, UpdatedAt: now},
		{UserID: "user-1", Dimension: domain.RankDimensionVolume, Value: decimal.NewFromInt(25000), Rank: 5, UpdatedAt: now},
		{UserID: "user-1", Dimension: domain.RankDimensionWinRate, Value: decimal.NewFromFloat(0.72), Rank: 10, UpdatedAt: now},
		{UserID: "user-1", Dimension: domain.RankDimensionTradeCount, Value: decimal.NewFromInt(150), Rank: 7, UpdatedAt: now},
	}

	env.rankingSvc.getUserRankingFn = func(_ context.Context, userID string) ([]*domain.UserRanking, error) {
		assert.Equal(t, "user-1", userID)
		return rankings, nil
	}

	resp, err := env.client.GetUserRanking(context.Background(), &rankingv1.GetUserRankingRequest{
		UserId: "user-1",
	})
	require.NoError(t, err)

	ranks := resp.GetRanks()
	require.Len(t, ranks, 5, "should return 5 dimension ranks")

	// Verify each rank has dimension, rank value, and decimal value.
	assert.Equal(t, rankingv1.RankDimension_RANK_DIMENSION_TOTAL_ASSETS, ranks[0].GetDimension())
	assert.Equal(t, int64(1), ranks[0].GetRank())
	assert.Equal(t, "5000", ranks[0].GetValue())

	assert.Equal(t, rankingv1.RankDimension_RANK_DIMENSION_PNL, ranks[1].GetDimension())
	assert.Equal(t, int64(3), ranks[1].GetRank())
	assert.Equal(t, "1200", ranks[1].GetValue())

	assert.Equal(t, rankingv1.RankDimension_RANK_DIMENSION_VOLUME, ranks[2].GetDimension())
	assert.Equal(t, int64(5), ranks[2].GetRank())
	assert.Equal(t, "25000", ranks[2].GetValue())

	assert.Equal(t, rankingv1.RankDimension_RANK_DIMENSION_WIN_RATE, ranks[3].GetDimension())
	assert.Equal(t, int64(10), ranks[3].GetRank())
	assert.Equal(t, "0.72", ranks[3].GetValue())

	assert.Equal(t, int64(7), ranks[4].GetRank())
	assert.Equal(t, "150", ranks[4].GetValue())
}

func TestGRPC_GetPortfolio_ReturnsPositions(t *testing.T) {
	env := newTestEnv(t)

	portfolio := &rankinggrpc.Portfolio{
		TotalValue:    decimal.NewFromInt(1050),
		UnrealizedPnL: decimal.NewFromInt(50),
		Positions: []rankinggrpc.PortfolioPosition{
			{
				MarketID:  "market-1",
				OutcomeID: "outcome-1",
				Quantity:  decimal.NewFromInt(10),
				AvgPrice:  decimal.NewFromFloat(0.50),
				Value:     decimal.NewFromInt(550),
			},
			{
				MarketID:  "market-2",
				OutcomeID: "outcome-3",
				Quantity:  decimal.NewFromInt(5),
				AvgPrice:  decimal.NewFromFloat(0.80),
				Value:     decimal.NewFromInt(500),
			},
		},
	}

	env.portfolioSvc.getPortfolioFn = func(_ context.Context, userID string) (*rankinggrpc.Portfolio, error) {
		assert.Equal(t, "user-1", userID)
		return portfolio, nil
	}

	resp, err := env.client.GetPortfolio(context.Background(), &rankingv1.GetPortfolioRequest{
		UserId: "user-1",
	})
	require.NoError(t, err)

	assert.Equal(t, "1050", resp.GetTotalValue())
	assert.Equal(t, "50", resp.GetUnrealizedPnl())

	positions := resp.GetPositions()
	require.Len(t, positions, 2, "should return 2 positions")

	first := positions[0]
	assert.Equal(t, "market-1", first.GetMarketId())
	assert.Equal(t, "outcome-1", first.GetOutcomeId())
	assert.Equal(t, "10", first.GetQuantity())
	assert.Equal(t, "0.5", first.GetAvgPrice())
}

func TestGRPC_RefreshRankings_ReturnsSuccess(t *testing.T) {
	env := newTestEnv(t)

	env.rankingSvc.refreshRankingsFn = func(_ context.Context) error {
		return nil
	}

	resp, err := env.client.RefreshRankings(context.Background(), &rankingv1.RefreshRankingsRequest{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.GetUpdatedCount(), int64(0))
}
