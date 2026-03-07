package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rankingv1 "github.com/truthmarket/truth-market/proto/gen/go/ranking/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockRankingClient implements rankingv1.RankingServiceClient for contract testing.
type mockRankingClient struct {
	getLeaderboardResp  *rankingv1.GetLeaderboardResponse
	getLeaderboardErr   error
	getUserRankingResp  *rankingv1.GetUserRankingResponse
	getUserRankingErr   error
	getPortfolioResp    *rankingv1.GetPortfolioResponse
	getPortfolioErr     error
	refreshRankingsResp *rankingv1.RefreshRankingsResponse
	refreshRankingsErr  error
}

func (m *mockRankingClient) GetLeaderboard(_ context.Context, _ *rankingv1.GetLeaderboardRequest, _ ...grpc.CallOption) (*rankingv1.GetLeaderboardResponse, error) {
	return m.getLeaderboardResp, m.getLeaderboardErr
}

func (m *mockRankingClient) GetUserRanking(_ context.Context, _ *rankingv1.GetUserRankingRequest, _ ...grpc.CallOption) (*rankingv1.GetUserRankingResponse, error) {
	return m.getUserRankingResp, m.getUserRankingErr
}

func (m *mockRankingClient) GetPortfolio(_ context.Context, _ *rankingv1.GetPortfolioRequest, _ ...grpc.CallOption) (*rankingv1.GetPortfolioResponse, error) {
	return m.getPortfolioResp, m.getPortfolioErr
}

func (m *mockRankingClient) RefreshRankings(_ context.Context, _ *rankingv1.RefreshRankingsRequest, _ ...grpc.CallOption) (*rankingv1.RefreshRankingsResponse, error) {
	return m.refreshRankingsResp, m.refreshRankingsErr
}

func setupContractRankingRouter(h *RankingHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rankings := r.Group("/api/v1/rankings")
	{
		rankings.GET("", h.GetLeaderboard)
		rankings.GET("/user/:id", h.GetUserRanking)
	}
	return r
}

// TestContract_GetLeaderboard verifies every JSON field in the leaderboard response.
// This catches field name mismatches like the wallet_address bug.
func TestContract_GetLeaderboard(t *testing.T) {
	mock := &mockRankingClient{
		getLeaderboardResp: &rankingv1.GetLeaderboardResponse{
			Rankings: []*rankingv1.UserRanking{
				{
					UserId:        "user-1",
					WalletAddress: "0xABCDEF1234567890",
					UserType:      "human",
					Rank:          1,
					Value:         "1500.50",
					UpdatedAt:     timestamppb.Now(),
				},
				{
					UserId:        "user-2",
					WalletAddress: "0x9876543210FEDCBA",
					UserType:      "agent",
					Rank:          2,
					Value:         "1200.00",
					UpdatedAt:     timestamppb.Now(),
				},
			},
			Total: 2,
		},
	}
	h := NewRankingHandler(mock)
	router := setupContractRankingRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rankings?dimension=total_assets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	// Parse and verify EVERY field in the response.
	var data struct {
		Rankings []struct {
			UserID        string `json:"user_id"`
			WalletAddress string `json:"wallet_address"`
			UserType      string `json:"user_type"`
			Rank          int64  `json:"rank"`
			Value         string `json:"value"`
		} `json:"rankings"`
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	require.Len(t, data.Rankings, 2)
	assert.Equal(t, int64(2), data.Total)

	// First ranking entry — verify every field is populated.
	r1 := data.Rankings[0]
	assert.Equal(t, "user-1", r1.UserID, "user_id field missing or wrong")
	assert.Equal(t, "0xABCDEF1234567890", r1.WalletAddress, "wallet_address field missing or wrong")
	assert.Equal(t, "human", r1.UserType, "user_type field missing or wrong")
	assert.Equal(t, int64(1), r1.Rank, "rank field missing or wrong")
	assert.Equal(t, "1500.50", r1.Value, "value field missing or wrong")

	// Second ranking entry.
	r2 := data.Rankings[1]
	assert.Equal(t, "user-2", r2.UserID)
	assert.Equal(t, "0x9876543210FEDCBA", r2.WalletAddress)
	assert.Equal(t, "agent", r2.UserType)
	assert.Equal(t, int64(2), r2.Rank)
	assert.Equal(t, "1200.00", r2.Value)

	// Also verify using raw JSON that fields exist with expected names.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "rankings", "top-level 'rankings' key missing")
	assert.Contains(t, raw, "total", "top-level 'total' key missing")
}

// TestContract_GetUserRanking verifies every JSON field in the user ranking response.
func TestContract_GetUserRanking(t *testing.T) {
	mock := &mockRankingClient{
		getUserRankingResp: &rankingv1.GetUserRankingResponse{
			Ranks: []*rankingv1.DimensionRank{
				{
					Dimension: rankingv1.RankDimension_RANK_DIMENSION_TOTAL_ASSETS,
					Rank:      3,
					Value:     "950.00",
				},
				{
					Dimension: rankingv1.RankDimension_RANK_DIMENSION_PNL,
					Rank:      7,
					Value:     "120.50",
				},
				{
					Dimension: rankingv1.RankDimension_RANK_DIMENSION_VOLUME,
					Rank:      1,
					Value:     "5000.00",
				},
				{
					Dimension: rankingv1.RankDimension_RANK_DIMENSION_WIN_RATE,
					Rank:      5,
					Value:     "0.75",
				},
			},
		},
	}
	h := NewRankingHandler(mock)
	router := setupContractRankingRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rankings/user/user-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var data struct {
		Ranks []struct {
			Dimension string `json:"dimension"`
			Rank      int64  `json:"rank"`
			Value     string `json:"value"`
		} `json:"ranks"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	require.Len(t, data.Ranks, 4)

	// Verify every dimension has all three required fields populated.
	for i, rank := range data.Ranks {
		assert.NotEmpty(t, rank.Dimension, "ranks[%d].dimension is empty", i)
		assert.NotZero(t, rank.Rank, "ranks[%d].rank is zero", i)
		assert.NotEmpty(t, rank.Value, "ranks[%d].value is empty", i)
	}

	// Check specific values.
	assert.Equal(t, int64(3), data.Ranks[0].Rank)
	assert.Equal(t, "950.00", data.Ranks[0].Value)

	// Verify raw JSON field names.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "ranks", "top-level 'ranks' key missing")
}

// TestContract_GetLeaderboard_EmptyResponse verifies the response shape with no data.
func TestContract_GetLeaderboard_EmptyResponse(t *testing.T) {
	mock := &mockRankingClient{
		getLeaderboardResp: &rankingv1.GetLeaderboardResponse{
			Rankings: nil,
			Total:    0,
		},
	}
	h := NewRankingHandler(mock)
	router := setupContractRankingRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rankings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "rankings")
	assert.Contains(t, raw, "total")
}
