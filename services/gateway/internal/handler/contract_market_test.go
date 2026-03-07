package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestContract_ListMarkets verifies every JSON field in the list markets response.
func TestContract_ListMarkets(t *testing.T) {
	now := timestamppb.Now()
	mock := &mockMarketClient{
		listMarketsResp: &marketv1.ListMarketsResponse{
			Markets: []*marketv1.Market{
				{
					Id:          "market-1",
					Title:       "Will BTC reach 100k?",
					Description: "Resolves YES if BTC hits $100k.",
					Category:    "crypto",
					Status:      marketv1.MarketStatus_MARKET_STATUS_OPEN,
					MarketType:  marketv1.MarketType_MARKET_TYPE_BINARY,
					CreatedBy:   "user-1",
					CreatedAt:   now,
					EndTime:     now,
					Outcomes: []*marketv1.Outcome{
						{Id: "o-1", MarketId: "market-1", Label: "Yes", Index: 0},
						{Id: "o-2", MarketId: "market-1", Label: "No", Index: 1},
					},
				},
			},
			Total: 1,
		},
	}
	h := NewMarketHandler(mock)
	router := setupMarketTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/markets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var data struct {
		Markets []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Category    string `json:"category"`
			Status      string `json:"status"`
			MarketType  string `json:"market_type"`
			CreatedBy   string `json:"created_by"`
			Volume      string `json:"volume"`
			Liquidity   string `json:"liquidity"`
			EndTime     string `json:"end_time"`
			CreatedAt   string `json:"created_at"`
			Outcomes    []struct {
				ID       string `json:"id"`
				Label    string `json:"label"`
				Index    int32  `json:"index"`
				Price    string `json:"price"`
				IsWinner bool   `json:"is_winner"`
			} `json:"outcomes"`
		} `json:"markets"`
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	require.Len(t, data.Markets, 1)
	assert.Equal(t, int64(1), data.Total)

	m := data.Markets[0]
	assert.Equal(t, "market-1", m.ID, "id field missing or wrong")
	assert.Equal(t, "Will BTC reach 100k?", m.Title, "title field missing or wrong")
	assert.Equal(t, "Resolves YES if BTC hits $100k.", m.Description, "description field missing or wrong")
	assert.Equal(t, "crypto", m.Category, "category field missing or wrong")
	assert.Equal(t, "open", m.Status, "status field missing or wrong")
	assert.Equal(t, "binary", m.MarketType, "market_type field missing or wrong")
	assert.Equal(t, "user-1", m.CreatedBy, "created_by field missing or wrong")
	assert.Equal(t, "0", m.Volume, "volume field missing or wrong")
	assert.Equal(t, "0", m.Liquidity, "liquidity field missing or wrong")
	assert.NotEmpty(t, m.EndTime, "end_time field missing")
	assert.NotEmpty(t, m.CreatedAt, "created_at field missing")

	// Verify outcomes.
	require.Len(t, m.Outcomes, 2)
	assert.Equal(t, "o-1", m.Outcomes[0].ID, "outcomes[0].id missing or wrong")
	assert.Equal(t, "Yes", m.Outcomes[0].Label, "outcomes[0].label missing or wrong")
	assert.Equal(t, int32(0), m.Outcomes[0].Index, "outcomes[0].index wrong")
	assert.Equal(t, "0.50", m.Outcomes[0].Price, "outcomes[0].price missing or wrong")
	assert.Equal(t, "o-2", m.Outcomes[1].ID)
	assert.Equal(t, "No", m.Outcomes[1].Label)
}

// TestContract_GetMarket verifies every JSON field in the get market response.
func TestContract_GetMarket(t *testing.T) {
	now := timestamppb.Now()
	mock := &mockMarketClient{
		getMarketResp: &marketv1.GetMarketResponse{
			Market: &marketv1.Market{
				Id:          "market-1",
				Title:       "Will BTC reach 100k?",
				Description: "Resolves YES if BTC hits $100k.",
				Category:    "crypto",
				Status:      marketv1.MarketStatus_MARKET_STATUS_OPEN,
				MarketType:  marketv1.MarketType_MARKET_TYPE_BINARY,
				CreatedBy:   "user-1",
				CreatedAt:   now,
				Outcomes: []*marketv1.Outcome{
					{Id: "o-1", MarketId: "market-1", Label: "Yes", Index: 0, IsWinner: false},
					{Id: "o-2", MarketId: "market-1", Label: "No", Index: 1, IsWinner: false},
				},
			},
		},
	}
	h := NewMarketHandler(mock)
	router := setupMarketTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/markets/market-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	// GetMarket wraps response in {"market": {...}}.
	var wrapper struct {
		Market struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Category    string `json:"category"`
			Status      string `json:"status"`
			MarketType  string `json:"market_type"`
			CreatedBy   string `json:"created_by"`
			Volume      string `json:"volume"`
			Liquidity   string `json:"liquidity"`
			CreatedAt   string `json:"created_at"`
			Outcomes    []struct {
				ID       string `json:"id"`
				Label    string `json:"label"`
				Index    int32  `json:"index"`
				Price    string `json:"price"`
				IsWinner bool   `json:"is_winner"`
			} `json:"outcomes"`
		} `json:"market"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &wrapper))

	m := wrapper.Market
	assert.Equal(t, "market-1", m.ID)
	assert.Equal(t, "Will BTC reach 100k?", m.Title)
	assert.Equal(t, "Resolves YES if BTC hits $100k.", m.Description)
	assert.Equal(t, "crypto", m.Category)
	assert.Equal(t, "open", m.Status)
	assert.Equal(t, "binary", m.MarketType)
	assert.Equal(t, "user-1", m.CreatedBy)
	assert.NotEmpty(t, m.CreatedAt, "created_at missing")
	require.Len(t, m.Outcomes, 2)
	assert.Equal(t, "o-1", m.Outcomes[0].ID)
	assert.Equal(t, "Yes", m.Outcomes[0].Label)
	assert.Equal(t, "0.50", m.Outcomes[0].Price)
	assert.False(t, m.Outcomes[0].IsWinner)
}

// TestContract_ListMarkets_ResponseShape verifies top-level response keys
// even when markets list is empty.
func TestContract_ListMarkets_ResponseShape(t *testing.T) {
	mock := &mockMarketClient{
		listMarketsResp: &marketv1.ListMarketsResponse{
			Markets: nil,
			Total:   0,
		},
	}
	h := NewMarketHandler(mock)
	router := setupMarketTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/markets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "markets", "top-level 'markets' key missing")
	assert.Contains(t, raw, "total", "top-level 'total' key missing")
}
