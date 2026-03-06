package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
	tradingv1 "github.com/truthmarket/truth-market/proto/gen/go/trading/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockTradingClient implements tradingv1.TradingServiceClient for testing.
type mockTradingClient struct {
	placeOrderResp              *tradingv1.PlaceOrderResponse
	placeOrderErr               error
	cancelOrderResp             *tradingv1.CancelOrderResponse
	cancelOrderErr              error
	getOrderResp                *tradingv1.GetOrderResponse
	getOrderErr                 error
	listOrdersResp              *tradingv1.ListOrdersResponse
	listOrdersErr               error
	mintTokensResp              *tradingv1.MintTokensResponse
	mintTokensErr               error
	getPositionsResp            *tradingv1.GetPositionsResponse
	getPositionsErr             error
	getOrderbookResp            *tradingv1.GetOrderbookResponse
	getOrderbookErr             error
	cancelAllOrdersByMarketResp *tradingv1.CancelAllOrdersByMarketResponse
	cancelAllOrdersByMarketErr  error
}

func (m *mockTradingClient) PlaceOrder(_ context.Context, _ *tradingv1.PlaceOrderRequest, _ ...grpc.CallOption) (*tradingv1.PlaceOrderResponse, error) {
	return m.placeOrderResp, m.placeOrderErr
}

func (m *mockTradingClient) CancelOrder(_ context.Context, _ *tradingv1.CancelOrderRequest, _ ...grpc.CallOption) (*tradingv1.CancelOrderResponse, error) {
	return m.cancelOrderResp, m.cancelOrderErr
}

func (m *mockTradingClient) GetOrder(_ context.Context, _ *tradingv1.GetOrderRequest, _ ...grpc.CallOption) (*tradingv1.GetOrderResponse, error) {
	return m.getOrderResp, m.getOrderErr
}

func (m *mockTradingClient) ListOrders(_ context.Context, _ *tradingv1.ListOrdersRequest, _ ...grpc.CallOption) (*tradingv1.ListOrdersResponse, error) {
	return m.listOrdersResp, m.listOrdersErr
}

func (m *mockTradingClient) MintTokens(_ context.Context, _ *tradingv1.MintTokensRequest, _ ...grpc.CallOption) (*tradingv1.MintTokensResponse, error) {
	return m.mintTokensResp, m.mintTokensErr
}

func (m *mockTradingClient) GetPositions(_ context.Context, _ *tradingv1.GetPositionsRequest, _ ...grpc.CallOption) (*tradingv1.GetPositionsResponse, error) {
	return m.getPositionsResp, m.getPositionsErr
}

func (m *mockTradingClient) GetOrderbook(_ context.Context, _ *tradingv1.GetOrderbookRequest, _ ...grpc.CallOption) (*tradingv1.GetOrderbookResponse, error) {
	return m.getOrderbookResp, m.getOrderbookErr
}

func (m *mockTradingClient) CancelAllOrdersByMarket(_ context.Context, _ *tradingv1.CancelAllOrdersByMarketRequest, _ ...grpc.CallOption) (*tradingv1.CancelAllOrdersByMarketResponse, error) {
	return m.cancelAllOrdersByMarketResp, m.cancelAllOrdersByMarketErr
}

func setupOrderTestRouter(h *OrderHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Simulate authenticated user in context.
	r.Use(func(c *gin.Context) {
		c.Set("user", &authv1.User{
			Id:            "user-1",
			WalletAddress: "0xUSER1",
			UserType:      authv1.UserType_USER_TYPE_HUMAN,
			IsAdmin:       false,
		})
		c.Next()
	})

	r.POST("/api/v1/orders", h.PlaceOrder)
	r.POST("/api/v1/markets/:id/mint", h.MintTokens)

	return r
}

func TestPlaceOrderHandler_ValidPayload_Returns201(t *testing.T) {
	mock := &mockTradingClient{
		placeOrderResp: &tradingv1.PlaceOrderResponse{
			Order: &tradingv1.Order{
				Id:        "order-new",
				UserId:    "user-1",
				MarketId:  "market-1",
				OutcomeId: "outcome-1",
				Side:      tradingv1.OrderSide_ORDER_SIDE_BUY,
				Price:     "0.5",
				Quantity:  "10",
				Status:    tradingv1.OrderStatus_ORDER_STATUS_OPEN,
			},
		},
	}
	h := &OrderHandler{tradingClient: mock}
	router := setupOrderTestRouter(h)

	body := map[string]interface{}{
		"market_id":  "market-1",
		"outcome_id": "outcome-1",
		"side":       "buy",
		"price":      "0.50",
		"quantity":   "10",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp jsonResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	var data struct {
		ID       string `json:"id"`
		Side     string `json:"side"`
		Price    string `json:"price"`
		Quantity string `json:"quantity"`
		Status   string `json:"status"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Equal(t, "order-new", data.ID)
	assert.Equal(t, "0.5", data.Price)
}

func TestPlaceOrderHandler_InvalidPrice_Returns400(t *testing.T) {
	mock := &mockTradingClient{
		placeOrderErr: status.Error(codes.InvalidArgument, "price must be between 0.01 and 0.99"),
	}
	h := &OrderHandler{tradingClient: mock}
	router := setupOrderTestRouter(h)

	body := map[string]interface{}{
		"market_id":  "market-1",
		"outcome_id": "outcome-1",
		"side":       "buy",
		"price":      "1.50",
		"quantity":   "10",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp jsonResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
}

func TestMintHandler_ValidPayload_Returns200(t *testing.T) {
	mock := &mockTradingClient{
		mintTokensResp: &tradingv1.MintTokensResponse{
			Positions: []*tradingv1.Position{
				{Id: "pos-1", UserId: "user-1", MarketId: "market-1", OutcomeId: "o-yes", Quantity: "10", AvgPrice: "1"},
				{Id: "pos-2", UserId: "user-1", MarketId: "market-1", OutcomeId: "o-no", Quantity: "10", AvgPrice: "1"},
			},
			Cost: "10",
		},
	}
	h := &OrderHandler{tradingClient: mock}
	router := setupOrderTestRouter(h)

	body := map[string]string{
		"quantity": "10",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/markets/market-1/mint", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	var data struct {
		Positions []struct {
			ID        string `json:"id"`
			OutcomeID string `json:"outcome_id"`
			Quantity  string `json:"quantity"`
		} `json:"positions"`
		Cost string `json:"cost"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Len(t, data.Positions, 2)
	assert.Equal(t, "10", data.Cost)
	assert.Equal(t, "o-yes", data.Positions[0].OutcomeID)
	assert.Equal(t, "o-no", data.Positions[1].OutcomeID)
}
