package grpc_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
	marketgrpc "github.com/truthmarket/truth-market/services/market-svc/internal/grpc"
)

// ---------------------------------------------------------------------------
// Mock services
// ---------------------------------------------------------------------------

// mockMarketService implements marketgrpc.MarketServicer for testing.
type mockMarketService struct {
	createMarketFn       func(ctx context.Context, req marketgrpc.CreateMarketRequest) (*domain.Market, error)
	getMarketFn          func(ctx context.Context, id string) (*domain.Market, []*domain.Outcome, error)
	listMarketsFn        func(ctx context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error)
	updateMarketStatusFn func(ctx context.Context, id string, status domain.MarketStatus) error
	resolveMarketFn      func(ctx context.Context, marketID, winningOutcomeID string) error
}

func (m *mockMarketService) CreateMarket(ctx context.Context, req marketgrpc.CreateMarketRequest) (*domain.Market, error) {
	return m.createMarketFn(ctx, req)
}

func (m *mockMarketService) GetMarket(ctx context.Context, id string) (*domain.Market, []*domain.Outcome, error) {
	return m.getMarketFn(ctx, id)
}

func (m *mockMarketService) ListMarkets(ctx context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error) {
	return m.listMarketsFn(ctx, filter)
}

func (m *mockMarketService) UpdateMarketStatus(ctx context.Context, id string, status domain.MarketStatus) error {
	return m.updateMarketStatusFn(ctx, id, status)
}

func (m *mockMarketService) ResolveMarket(ctx context.Context, marketID, winningOutcomeID string) error {
	return m.resolveMarketFn(ctx, marketID, winningOutcomeID)
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

const bufSize = 1024 * 1024

// testEnv bundles a bufconn-based gRPC client together with the mock service
// so each test can configure behaviour and make real gRPC calls.
type testEnv struct {
	client     marketv1.MarketServiceClient
	marketSvc  *mockMarketService
	conn       *grpc.ClientConn
	grpcServer *grpc.Server
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	marketSvc := &mockMarketService{}

	srv := marketgrpc.NewMarketServer(marketSvc)

	lis := bufconn.Listen(bufSize)
	gs := grpc.NewServer()
	marketv1.RegisterMarketServiceServer(gs, srv)

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
		client:     marketv1.NewMarketServiceClient(conn),
		marketSvc:  marketSvc,
		conn:       conn,
		grpcServer: gs,
	}
}

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

func testMarket() *domain.Market {
	closesAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return &domain.Market{
		ID:          "market-123",
		Title:       "Will BTC exceed $100k by end of 2026?",
		Description: "Resolves YES if Bitcoin price exceeds $100,000 on any major exchange.",
		Category:    "crypto",
		MarketType:  domain.MarketTypeBinary,
		Status:      domain.MarketStatusDraft,
		CreatorID:   "user-1",
		CreatedAt:   time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		ClosesAt:    &closesAt,
	}
}

func testOutcomes() []*domain.Outcome {
	return []*domain.Outcome{
		{ID: "o-yes", MarketID: "market-123", Label: "Yes", Index: 0, IsWinner: false},
		{ID: "o-no", MarketID: "market-123", Label: "No", Index: 1, IsWinner: false},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGRPC_CreateMarket_Returns201(t *testing.T) {
	env := newTestEnv(t)

	m := testMarket()
	env.marketSvc.createMarketFn = func(_ context.Context, req marketgrpc.CreateMarketRequest) (*domain.Market, error) {
		assert.Equal(t, "Will BTC exceed $100k by end of 2026?", req.Title)
		assert.Equal(t, "Resolves YES if Bitcoin price exceeds $100,000 on any major exchange.", req.Description)
		assert.Equal(t, domain.MarketTypeBinary, req.MarketType)
		assert.Equal(t, "crypto", req.Category)
		assert.Equal(t, []string{"Yes", "No"}, req.OutcomeLabels)
		assert.Equal(t, "user-1", req.CreatedBy)
		return m, nil
	}

	endTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	resp, err := env.client.CreateMarket(context.Background(), &marketv1.CreateMarketRequest{
		Title:         "Will BTC exceed $100k by end of 2026?",
		Description:   "Resolves YES if Bitcoin price exceeds $100,000 on any major exchange.",
		MarketType:    marketv1.MarketType_MARKET_TYPE_BINARY,
		Category:      "crypto",
		OutcomeLabels: []string{"Yes", "No"},
		EndTime:       timestamppb.New(endTime),
		CreatedBy:     "user-1",
	})
	require.NoError(t, err)

	assert.Equal(t, m.ID, resp.GetMarket().GetId())
	assert.Equal(t, m.Title, resp.GetMarket().GetTitle())
	assert.Equal(t, m.Description, resp.GetMarket().GetDescription())
	assert.Equal(t, marketv1.MarketType_MARKET_TYPE_BINARY, resp.GetMarket().GetMarketType())
	assert.Equal(t, m.Category, resp.GetMarket().GetCategory())
	assert.Equal(t, marketv1.MarketStatus_MARKET_STATUS_DRAFT, resp.GetMarket().GetStatus())
	assert.Equal(t, m.CreatorID, resp.GetMarket().GetCreatedBy())
}

func TestGRPC_GetMarket_ReturnsMarketWithOutcomes(t *testing.T) {
	env := newTestEnv(t)

	m := testMarket()
	outcomes := testOutcomes()
	env.marketSvc.getMarketFn = func(_ context.Context, id string) (*domain.Market, []*domain.Outcome, error) {
		assert.Equal(t, "market-123", id)
		return m, outcomes, nil
	}

	resp, err := env.client.GetMarket(context.Background(), &marketv1.GetMarketRequest{
		MarketId: "market-123",
	})
	require.NoError(t, err)

	assert.Equal(t, m.ID, resp.GetMarket().GetId())
	assert.Equal(t, m.Title, resp.GetMarket().GetTitle())
	assert.Equal(t, m.Description, resp.GetMarket().GetDescription())
	assert.Equal(t, marketv1.MarketType_MARKET_TYPE_BINARY, resp.GetMarket().GetMarketType())
	assert.Equal(t, m.Category, resp.GetMarket().GetCategory())
	assert.Equal(t, marketv1.MarketStatus_MARKET_STATUS_DRAFT, resp.GetMarket().GetStatus())
	assert.Equal(t, m.CreatorID, resp.GetMarket().GetCreatedBy())

	// Verify outcomes are included in the response.
	protoOutcomes := resp.GetMarket().GetOutcomes()
	require.Len(t, protoOutcomes, 2, "response should include 2 outcomes")
	assert.Equal(t, "o-yes", protoOutcomes[0].GetId())
	assert.Equal(t, "Yes", protoOutcomes[0].GetLabel())
	assert.Equal(t, int32(0), protoOutcomes[0].GetIndex())
	assert.Equal(t, "o-no", protoOutcomes[1].GetId())
	assert.Equal(t, "No", protoOutcomes[1].GetLabel())
	assert.Equal(t, int32(1), protoOutcomes[1].GetIndex())
}

func TestGRPC_GetMarket_NotFound_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	env.marketSvc.getMarketFn = func(_ context.Context, _ string) (*domain.Market, []*domain.Outcome, error) {
		return nil, nil, apperrors.Wrap(fmt.Errorf("no rows"), "NOT_FOUND", "market not found")
	}

	resp, err := env.client.GetMarket(context.Background(), &marketv1.GetMarketRequest{
		MarketId: "nonexistent",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Equal(t, "market not found", st.Message())
}

func TestGRPC_ListMarkets_ReturnsPaginated(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	markets := []*domain.Market{
		{
			ID: "m-1", Title: "Market 1", Status: domain.MarketStatusOpen,
			MarketType: domain.MarketTypeBinary, Category: "crypto",
			CreatorID: "user-1", CreatedAt: now,
		},
		{
			ID: "m-2", Title: "Market 2", Status: domain.MarketStatusOpen,
			MarketType: domain.MarketTypeMulti, Category: "sports",
			CreatorID: "user-2", CreatedAt: now.Add(1 * time.Second),
		},
	}

	env.marketSvc.listMarketsFn = func(_ context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error) {
		assert.Equal(t, int(1), filter.Limit)
		assert.Equal(t, int(0), filter.Offset)
		return markets[:1], int64(5), nil
	}

	// GetMarket is called for each listed market to fetch outcomes.
	env.marketSvc.getMarketFn = func(_ context.Context, id string) (*domain.Market, []*domain.Outcome, error) {
		assert.Equal(t, "m-1", id)
		return markets[0], []*domain.Outcome{
			{ID: "o-yes", MarketID: "m-1", Label: "Yes", Index: 0},
			{ID: "o-no", MarketID: "m-1", Label: "No", Index: 1},
		}, nil
	}

	resp, err := env.client.ListMarkets(context.Background(), &marketv1.ListMarketsRequest{
		Status:  marketv1.MarketStatus_MARKET_STATUS_OPEN,
		Page:    1,
		PerPage: 1,
	})
	require.NoError(t, err)

	require.Len(t, resp.GetMarkets(), 1, "should return 1 market for page size 1")
	assert.Equal(t, "m-1", resp.GetMarkets()[0].GetId())
	assert.Equal(t, int64(5), resp.GetTotal(), "total should reflect all matching markets")

	// Verify outcomes are included in listed markets.
	protoOutcomes := resp.GetMarkets()[0].GetOutcomes()
	require.Len(t, protoOutcomes, 2, "listed market should include outcomes")
	assert.Equal(t, "Yes", protoOutcomes[0].GetLabel())
	assert.Equal(t, "No", protoOutcomes[1].GetLabel())
}

func TestGRPC_ResolveMarket_Success(t *testing.T) {
	env := newTestEnv(t)

	env.marketSvc.resolveMarketFn = func(_ context.Context, marketID, winningOutcomeID string) error {
		assert.Equal(t, "market-123", marketID)
		assert.Equal(t, "o-yes", winningOutcomeID)
		return nil
	}

	resp, err := env.client.ResolveMarket(context.Background(), &marketv1.ResolveMarketRequest{
		MarketId:         "market-123",
		WinningOutcomeId: "o-yes",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGRPC_ResolveMarket_NotClosed_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	env.marketSvc.resolveMarketFn = func(_ context.Context, _, _ string) error {
		return apperrors.Wrap(fmt.Errorf("market is not closed"), "BAD_REQUEST", "market must be closed before resolution")
	}

	resp, err := env.client.ResolveMarket(context.Background(), &marketv1.ResolveMarketRequest{
		MarketId:         "market-open",
		WinningOutcomeId: "o-a",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Equal(t, "market must be closed before resolution", st.Message())
}
