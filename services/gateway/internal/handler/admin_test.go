package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
)

func setupAdminTestRouter(h *AdminHandler, isAdmin bool, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Simulate auth middleware by setting user and is_admin in context.
	r.Use(func(c *gin.Context) {
		c.Set("user", &authv1.User{
			Id:            userID,
			WalletAddress: "0xADMIN",
			UserType:      authv1.UserType_USER_TYPE_HUMAN,
			IsAdmin:       isAdmin,
		})
		c.Set("is_admin", isAdmin)
		c.Next()
	})

	admin := r.Group("/api/v1/admin/markets")
	{
		admin.POST("", h.CreateMarket)
		admin.POST("/:id/resolve", h.ResolveMarket)
	}

	return r
}

func TestCreateMarketHandler_AdminOnly(t *testing.T) {
	mock := &mockMarketClient{}
	h := &AdminHandler{marketClient: mock}
	router := setupAdminTestRouter(h, false, "user-regular")

	body := map[string]interface{}{
		"title":          "Will ETH merge succeed?",
		"description":    "Resolves YES if the Ethereum merge completes successfully.",
		"market_type":    "MARKET_TYPE_BINARY",
		"category":       "crypto",
		"outcome_labels": []string{"Yes", "No"},
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/markets", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp jsonResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
}

func TestCreateMarketHandler_Success(t *testing.T) {
	mock := &mockMarketClient{
		createMarketResp: &marketv1.CreateMarketResponse{
			Market: &marketv1.Market{
				Id:          "market-new",
				Title:       "Will ETH merge succeed?",
				Description: "Resolves YES if the Ethereum merge completes successfully.",
				Category:    "crypto",
				Status:      marketv1.MarketStatus_MARKET_STATUS_DRAFT,
				MarketType:  marketv1.MarketType_MARKET_TYPE_BINARY,
				CreatedBy:   "admin-1",
				Outcomes: []*marketv1.Outcome{
					{Id: "outcome-1", MarketId: "market-new", Label: "Yes", Index: 0},
					{Id: "outcome-2", MarketId: "market-new", Label: "No", Index: 1},
				},
			},
		},
	}
	h := &AdminHandler{marketClient: mock}
	router := setupAdminTestRouter(h, true, "admin-1")

	body := map[string]interface{}{
		"title":          "Will ETH merge succeed?",
		"description":    "Resolves YES if the Ethereum merge completes successfully.",
		"market_type":    "MARKET_TYPE_BINARY",
		"category":       "crypto",
		"outcome_labels": []string{"Yes", "No"},
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/markets", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp jsonResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	// Parse the data field to verify the created market.
	var data struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Outcomes    []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"outcomes"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Equal(t, "market-new", data.ID)
	assert.Equal(t, "Will ETH merge succeed?", data.Title)
	assert.Equal(t, "crypto", data.Category)
	assert.Len(t, data.Outcomes, 2)
	assert.Equal(t, "Yes", data.Outcomes[0].Label)
	assert.Equal(t, "No", data.Outcomes[1].Label)
}

func TestResolveMarketHandler_AdminOnly(t *testing.T) {
	mock := &mockMarketClient{}
	h := &AdminHandler{marketClient: mock}
	router := setupAdminTestRouter(h, false, "user-regular")

	body := map[string]string{
		"winning_outcome_id": "outcome-1",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/markets/market-1/resolve", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp jsonResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
}

func TestResolveMarketHandler_ValidPayload(t *testing.T) {
	resolvedMarket := &marketv1.Market{
		Id:         "market-1",
		Title:      "Will BTC reach 100k?",
		Category:   "crypto",
		Status:     marketv1.MarketStatus_MARKET_STATUS_RESOLVED,
		MarketType: marketv1.MarketType_MARKET_TYPE_BINARY,
		CreatedBy:  "user-1",
		Outcomes: []*marketv1.Outcome{
			{Id: "outcome-1", MarketId: "market-1", Label: "Yes", Index: 0, IsWinner: true},
			{Id: "outcome-2", MarketId: "market-1", Label: "No", Index: 1, IsWinner: false},
		},
	}
	mock := &mockMarketClient{
		resolveMarketResp: &marketv1.ResolveMarketResponse{Market: resolvedMarket},
		getMarketResp:     &marketv1.GetMarketResponse{Market: resolvedMarket},
	}
	h := &AdminHandler{marketClient: mock}
	router := setupAdminTestRouter(h, true, "admin-1")

	body := map[string]string{
		"winning_outcome_id": "outcome-1",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/markets/market-1/resolve", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	// Parse the data field to verify resolved market.
	var data struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Outcomes []struct {
			ID       string `json:"id"`
			Label    string `json:"label"`
			IsWinner bool   `json:"is_winner"`
		} `json:"outcomes"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Equal(t, "market-1", data.ID)
	assert.Len(t, data.Outcomes, 2)
	assert.True(t, data.Outcomes[0].IsWinner)
	assert.False(t, data.Outcomes[1].IsWinner)
}
