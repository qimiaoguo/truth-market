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
	tradingv1 "github.com/truthmarket/truth-market/proto/gen/go/trading/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupContractTradingRouter(h *OrderHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &authv1.User{
			Id:            "user-1",
			WalletAddress: "0xCONTRACT_TEST",
			UserType:      authv1.UserType_USER_TYPE_HUMAN,
		})
		c.Next()
	})

	r.POST("/api/v1/orders", h.PlaceOrder)
	r.GET("/api/v1/trading/orders", h.ListOrders)
	r.GET("/api/v1/trading/positions", h.GetPositions)
	r.GET("/api/v1/markets/:id/orderbook", h.GetOrderbook)
	r.POST("/api/v1/markets/:id/mint", h.MintTokens)
	return r
}

// TestContract_PlaceOrder verifies every JSON field in the place order response.
func TestContract_PlaceOrder(t *testing.T) {
	now := timestamppb.Now()
	mock := &mockTradingClient{
		placeOrderResp: &tradingv1.PlaceOrderResponse{
			Order: &tradingv1.Order{
				Id:             "order-1",
				UserId:         "user-1",
				MarketId:       "market-1",
				OutcomeId:      "outcome-1",
				Side:           tradingv1.OrderSide_ORDER_SIDE_BUY,
				Price:          "0.65",
				Quantity:       "100",
				FilledQuantity: "0",
				Status:         tradingv1.OrderStatus_ORDER_STATUS_OPEN,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			Trades: []*tradingv1.Trade{
				{
					Id:           "trade-1",
					MarketId:     "market-1",
					OutcomeId:    "outcome-1",
					MakerOrderId: "order-0",
					TakerOrderId: "order-1",
					MakerUserId:  "user-2",
					TakerUserId:  "user-1",
					Price:        "0.65",
					Quantity:     "50",
					CreatedAt:    now,
				},
			},
		},
	}
	h := NewOrderHandler(mock)
	router := setupContractTradingRouter(h)

	body, _ := json.Marshal(map[string]string{
		"market_id":  "market-1",
		"outcome_id": "outcome-1",
		"side":       "buy",
		"price":      "0.65",
		"quantity":   "100",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	// PlaceOrder returns flat order fields + optional trades array.
	var data struct {
		ID             string `json:"id"`
		UserID         string `json:"user_id"`
		MarketID       string `json:"market_id"`
		OutcomeID      string `json:"outcome_id"`
		Side           string `json:"side"`
		Price          string `json:"price"`
		Quantity       string `json:"quantity"`
		FilledQuantity string `json:"filled_quantity"`
		Status         string `json:"status"`
		Trades         []struct {
			ID           string `json:"id"`
			MakerOrderID string `json:"maker_order_id"`
			TakerOrderID string `json:"taker_order_id"`
			Price        string `json:"price"`
			Quantity     string `json:"quantity"`
		} `json:"trades"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	assert.Equal(t, "order-1", data.ID, "id field missing or wrong")
	assert.Equal(t, "user-1", data.UserID, "user_id field missing or wrong")
	assert.Equal(t, "market-1", data.MarketID, "market_id field missing or wrong")
	assert.Equal(t, "outcome-1", data.OutcomeID, "outcome_id field missing or wrong")
	assert.Equal(t, "buy", data.Side, "side field missing or wrong")
	assert.Equal(t, "0.65", data.Price, "price field missing or wrong")
	assert.Equal(t, "100", data.Quantity, "quantity field missing or wrong")
	assert.Equal(t, "0", data.FilledQuantity, "filled_quantity field missing or wrong")
	assert.Equal(t, "open", data.Status, "status field missing or wrong")

	// Verify trades.
	require.Len(t, data.Trades, 1)
	assert.Equal(t, "trade-1", data.Trades[0].ID, "trades[0].id missing or wrong")
	assert.Equal(t, "order-0", data.Trades[0].MakerOrderID, "trades[0].maker_order_id missing or wrong")
	assert.Equal(t, "order-1", data.Trades[0].TakerOrderID, "trades[0].taker_order_id missing or wrong")
	assert.Equal(t, "0.65", data.Trades[0].Price, "trades[0].price missing or wrong")
	assert.Equal(t, "50", data.Trades[0].Quantity, "trades[0].quantity missing or wrong")
}

// TestContract_ListOrders verifies every JSON field in the list orders response.
func TestContract_ListOrders(t *testing.T) {
	now := timestamppb.Now()
	mock := &mockTradingClient{
		listOrdersResp: &tradingv1.ListOrdersResponse{
			Orders: []*tradingv1.Order{
				{
					Id:             "order-1",
					UserId:         "user-1",
					MarketId:       "market-1",
					OutcomeId:      "outcome-1",
					Side:           tradingv1.OrderSide_ORDER_SIDE_BUY,
					Price:          "0.50",
					Quantity:       "10",
					FilledQuantity: "5",
					Status:         tradingv1.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED,
					CreatedAt:      now,
					UpdatedAt:      now,
				},
			},
			Total: 1,
		},
	}
	h := NewOrderHandler(mock)
	router := setupContractTradingRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trading/orders", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var data struct {
		Orders []struct {
			ID             string `json:"id"`
			UserID         string `json:"user_id"`
			MarketID       string `json:"market_id"`
			OutcomeID      string `json:"outcome_id"`
			Side           string `json:"side"`
			Price          string `json:"price"`
			Quantity       string `json:"quantity"`
			FilledQuantity string `json:"filled_quantity"`
			Status         string `json:"status"`
		} `json:"orders"`
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	require.Len(t, data.Orders, 1)
	assert.Equal(t, int64(1), data.Total)

	o := data.Orders[0]
	assert.Equal(t, "order-1", o.ID, "orders[0].id missing or wrong")
	assert.Equal(t, "user-1", o.UserID, "orders[0].user_id missing or wrong")
	assert.Equal(t, "market-1", o.MarketID, "orders[0].market_id missing or wrong")
	assert.Equal(t, "outcome-1", o.OutcomeID, "orders[0].outcome_id missing or wrong")
	assert.Equal(t, "buy", o.Side, "orders[0].side missing or wrong")
	assert.Equal(t, "0.50", o.Price, "orders[0].price missing or wrong")
	assert.Equal(t, "10", o.Quantity, "orders[0].quantity missing or wrong")
	assert.Equal(t, "5", o.FilledQuantity, "orders[0].filled_quantity missing or wrong")
	assert.Equal(t, "partial", o.Status, "orders[0].status missing or wrong")

	// Verify top-level keys.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "orders")
	assert.Contains(t, raw, "total")
}

// TestContract_GetPositions verifies every JSON field in the positions response.
func TestContract_GetPositions(t *testing.T) {
	now := timestamppb.Now()
	mock := &mockTradingClient{
		getPositionsResp: &tradingv1.GetPositionsResponse{
			Positions: []*tradingv1.Position{
				{
					Id:        "pos-1",
					UserId:    "user-1",
					MarketId:  "market-1",
					OutcomeId: "outcome-1",
					Quantity:  "50",
					AvgPrice:  "0.65",
					UpdatedAt: now,
				},
			},
		},
	}
	h := NewOrderHandler(mock)
	router := setupContractTradingRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trading/positions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var data struct {
		Positions []struct {
			ID        string `json:"id"`
			UserID    string `json:"user_id"`
			MarketID  string `json:"market_id"`
			OutcomeID string `json:"outcome_id"`
			Quantity  string `json:"quantity"`
			AvgPrice  string `json:"avg_price"`
		} `json:"positions"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	require.Len(t, data.Positions, 1)
	p := data.Positions[0]
	assert.Equal(t, "pos-1", p.ID, "positions[0].id missing or wrong")
	assert.Equal(t, "user-1", p.UserID, "positions[0].user_id missing or wrong")
	assert.Equal(t, "market-1", p.MarketID, "positions[0].market_id missing or wrong")
	assert.Equal(t, "outcome-1", p.OutcomeID, "positions[0].outcome_id missing or wrong")
	assert.Equal(t, "50", p.Quantity, "positions[0].quantity missing or wrong")
	assert.Equal(t, "0.65", p.AvgPrice, "positions[0].avg_price missing or wrong")
}

// TestContract_GetOrderbook verifies every JSON field in the orderbook response.
func TestContract_GetOrderbook(t *testing.T) {
	mock := &mockTradingClient{
		getOrderbookResp: &tradingv1.GetOrderbookResponse{
			Bids: []*tradingv1.OrderbookLevel{
				{Price: "0.60", Quantity: "100", OrderCount: 3},
				{Price: "0.55", Quantity: "200", OrderCount: 5},
			},
			Asks: []*tradingv1.OrderbookLevel{
				{Price: "0.65", Quantity: "80", OrderCount: 2},
			},
		},
	}
	h := NewOrderHandler(mock)
	router := setupContractTradingRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/markets/market-1/orderbook?outcome_id=outcome-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var data struct {
		Bids []struct {
			Price      string `json:"price"`
			Quantity   string `json:"quantity"`
			OrderCount int32  `json:"order_count"`
		} `json:"bids"`
		Asks []struct {
			Price      string `json:"price"`
			Quantity   string `json:"quantity"`
			OrderCount int32  `json:"order_count"`
		} `json:"asks"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	require.Len(t, data.Bids, 2)
	require.Len(t, data.Asks, 1)

	// Verify bid fields.
	assert.Equal(t, "0.60", data.Bids[0].Price, "bids[0].price missing or wrong")
	assert.Equal(t, "100", data.Bids[0].Quantity, "bids[0].quantity missing or wrong")
	assert.Equal(t, int32(3), data.Bids[0].OrderCount, "bids[0].order_count missing or wrong")

	// Verify ask fields.
	assert.Equal(t, "0.65", data.Asks[0].Price, "asks[0].price missing or wrong")
	assert.Equal(t, "80", data.Asks[0].Quantity, "asks[0].quantity missing or wrong")
	assert.Equal(t, int32(2), data.Asks[0].OrderCount, "asks[0].order_count missing or wrong")

	// Verify top-level keys.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "bids")
	assert.Contains(t, raw, "asks")
}

// TestContract_MintTokens verifies every JSON field in the mint response.
func TestContract_MintTokens(t *testing.T) {
	now := timestamppb.Now()
	mock := &mockTradingClient{
		mintTokensResp: &tradingv1.MintTokensResponse{
			Positions: []*tradingv1.Position{
				{
					Id:        "pos-yes",
					UserId:    "user-1",
					MarketId:  "market-1",
					OutcomeId: "outcome-yes",
					Quantity:  "100",
					AvgPrice:  "1",
					UpdatedAt: now,
				},
				{
					Id:        "pos-no",
					UserId:    "user-1",
					MarketId:  "market-1",
					OutcomeId: "outcome-no",
					Quantity:  "100",
					AvgPrice:  "1",
					UpdatedAt: now,
				},
			},
			Cost: "100",
		},
	}
	h := NewOrderHandler(mock)
	router := setupContractTradingRouter(h)

	body, _ := json.Marshal(map[string]string{
		"market_id": "market-1",
		"quantity":  "100",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/markets/market-1/mint", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var data struct {
		Positions []struct {
			ID        string `json:"id"`
			UserID    string `json:"user_id"`
			MarketID  string `json:"market_id"`
			OutcomeID string `json:"outcome_id"`
			Quantity  string `json:"quantity"`
			AvgPrice  string `json:"avg_price"`
		} `json:"positions"`
		Cost string `json:"cost"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	require.Len(t, data.Positions, 2)
	assert.Equal(t, "100", data.Cost, "cost field missing or wrong")

	p := data.Positions[0]
	assert.Equal(t, "pos-yes", p.ID, "positions[0].id missing or wrong")
	assert.Equal(t, "user-1", p.UserID, "positions[0].user_id missing or wrong")
	assert.Equal(t, "market-1", p.MarketID, "positions[0].market_id missing or wrong")
	assert.Equal(t, "outcome-yes", p.OutcomeID, "positions[0].outcome_id missing or wrong")
	assert.Equal(t, "100", p.Quantity, "positions[0].quantity missing or wrong")
	assert.Equal(t, "1", p.AvgPrice, "positions[0].avg_price missing or wrong")

	// Verify top-level keys.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "positions")
	assert.Contains(t, raw, "cost")
}
