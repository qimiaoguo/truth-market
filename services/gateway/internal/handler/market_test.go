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
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockMarketClient implements marketv1.MarketServiceClient for testing.
type mockMarketClient struct {
	createMarketResp       *marketv1.CreateMarketResponse
	createMarketErr        error
	getMarketResp          *marketv1.GetMarketResponse
	getMarketErr           error
	listMarketsResp        *marketv1.ListMarketsResponse
	listMarketsErr         error
	updateMarketStatusResp *marketv1.UpdateMarketStatusResponse
	updateMarketStatusErr  error
	resolveMarketResp      *marketv1.ResolveMarketResponse
	resolveMarketErr       error
	cancelMarketResp       *marketv1.CancelMarketResponse
	cancelMarketErr        error
}

func (m *mockMarketClient) CreateMarket(_ context.Context, _ *marketv1.CreateMarketRequest, _ ...grpc.CallOption) (*marketv1.CreateMarketResponse, error) {
	return m.createMarketResp, m.createMarketErr
}

func (m *mockMarketClient) GetMarket(_ context.Context, _ *marketv1.GetMarketRequest, _ ...grpc.CallOption) (*marketv1.GetMarketResponse, error) {
	return m.getMarketResp, m.getMarketErr
}

func (m *mockMarketClient) ListMarkets(_ context.Context, _ *marketv1.ListMarketsRequest, _ ...grpc.CallOption) (*marketv1.ListMarketsResponse, error) {
	return m.listMarketsResp, m.listMarketsErr
}

func (m *mockMarketClient) UpdateMarketStatus(_ context.Context, _ *marketv1.UpdateMarketStatusRequest, _ ...grpc.CallOption) (*marketv1.UpdateMarketStatusResponse, error) {
	return m.updateMarketStatusResp, m.updateMarketStatusErr
}

func (m *mockMarketClient) ResolveMarket(_ context.Context, _ *marketv1.ResolveMarketRequest, _ ...grpc.CallOption) (*marketv1.ResolveMarketResponse, error) {
	return m.resolveMarketResp, m.resolveMarketErr
}

func (m *mockMarketClient) CancelMarket(_ context.Context, _ *marketv1.CancelMarketRequest, _ ...grpc.CallOption) (*marketv1.CancelMarketResponse, error) {
	return m.cancelMarketResp, m.cancelMarketErr
}

func setupMarketTestRouter(h *MarketHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	markets := r.Group("/api/v1/markets")
	{
		markets.GET("", h.ListMarkets)
		markets.GET("/:id", h.GetMarket)
	}

	return r
}

func TestListMarketsHandler_PublicAccess(t *testing.T) {
	mock := &mockMarketClient{
		listMarketsResp: &marketv1.ListMarketsResponse{
			Markets: []*marketv1.Market{
				{
					Id:         "market-1",
					Title:      "Will BTC reach 100k?",
					Category:   "crypto",
					Status:     marketv1.MarketStatus_MARKET_STATUS_OPEN,
					MarketType: marketv1.MarketType_MARKET_TYPE_BINARY,
					CreatedBy:  "user-1",
					Outcomes: []*marketv1.Outcome{
						{Id: "outcome-1", MarketId: "market-1", Label: "Yes", Index: 0},
						{Id: "outcome-2", MarketId: "market-1", Label: "No", Index: 1},
					},
				},
				{
					Id:         "market-2",
					Title:      "2024 US Election Winner",
					Category:   "politics",
					Status:     marketv1.MarketStatus_MARKET_STATUS_OPEN,
					MarketType: marketv1.MarketType_MARKET_TYPE_MULTI,
					CreatedBy:  "user-2",
				},
			},
			Total: 2,
		},
	}
	h := &MarketHandler{marketClient: mock}
	router := setupMarketTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/markets", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	// Parse the data field to verify markets are present.
	var data struct {
		Markets []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Category string `json:"category"`
		} `json:"markets"`
		Total int64 `json:"total"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Len(t, data.Markets, 2)
	assert.Equal(t, int64(2), data.Total)
	assert.Equal(t, "market-1", data.Markets[0].ID)
	assert.Equal(t, "Will BTC reach 100k?", data.Markets[0].Title)
	assert.Equal(t, "crypto", data.Markets[0].Category)
	assert.Equal(t, "market-2", data.Markets[1].ID)
}

func TestGetMarketHandler_ReturnsMarketDetail(t *testing.T) {
	mock := &mockMarketClient{
		getMarketResp: &marketv1.GetMarketResponse{
			Market: &marketv1.Market{
				Id:          "market-1",
				Title:       "Will BTC reach 100k?",
				Description: "Resolves YES if BTC price reaches $100,000 before end date.",
				Category:    "crypto",
				Status:      marketv1.MarketStatus_MARKET_STATUS_OPEN,
				MarketType:  marketv1.MarketType_MARKET_TYPE_BINARY,
				CreatedBy:   "user-1",
				Outcomes: []*marketv1.Outcome{
					{Id: "outcome-1", MarketId: "market-1", Label: "Yes", Index: 0},
					{Id: "outcome-2", MarketId: "market-1", Label: "No", Index: 1},
				},
			},
		},
	}
	h := &MarketHandler{marketClient: mock}
	router := setupMarketTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/markets/market-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	// Parse the data field to verify market detail with outcomes.
	var data struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Outcomes    []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			Index int32  `json:"index"`
		} `json:"outcomes"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Equal(t, "market-1", data.ID)
	assert.Equal(t, "Will BTC reach 100k?", data.Title)
	assert.Equal(t, "Resolves YES if BTC price reaches $100,000 before end date.", data.Description)
	assert.Equal(t, "crypto", data.Category)
	assert.Len(t, data.Outcomes, 2)
	assert.Equal(t, "outcome-1", data.Outcomes[0].ID)
	assert.Equal(t, "Yes", data.Outcomes[0].Label)
	assert.Equal(t, "outcome-2", data.Outcomes[1].ID)
	assert.Equal(t, "No", data.Outcomes[1].Label)
}

func TestGetMarketHandler_NotFound(t *testing.T) {
	mock := &mockMarketClient{
		getMarketErr: status.Error(codes.NotFound, "market not found"),
	}
	h := &MarketHandler{marketClient: mock}
	router := setupMarketTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/markets/nonexistent-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
}
