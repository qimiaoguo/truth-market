package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
	tradingv1 "github.com/truthmarket/truth-market/proto/gen/go/trading/v1"
)

// OrderHandler handles HTTP requests for trading endpoints and
// delegates to the trading-svc via gRPC.
type OrderHandler struct {
	tradingClient tradingv1.TradingServiceClient
}

// NewOrderHandler creates a new OrderHandler with the given gRPC trading client.
func NewOrderHandler(tradingClient tradingv1.TradingServiceClient) *OrderHandler {
	return &OrderHandler{tradingClient: tradingClient}
}

// placeOrderRequest is the expected JSON body for the PlaceOrder endpoint.
type placeOrderRequest struct {
	MarketID  string `json:"market_id"`
	OutcomeID string `json:"outcome_id"`
	Side      string `json:"side"`
	Price     string `json:"price"`
	Quantity  string `json:"quantity"`
}

// PlaceOrder handles POST /api/v1/orders.
func (h *OrderHandler) PlaceOrder(c *gin.Context) {
	var req placeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := "invalid request body"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	// Get the authenticated user from the context.
	userVal, _ := c.Get("user")
	user, ok := userVal.(*authv1.User)
	if !ok || user == nil {
		errMsg := "unauthorized"
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	// Map side string to proto enum.
	var side tradingv1.OrderSide
	switch req.Side {
	case "buy":
		side = tradingv1.OrderSide_ORDER_SIDE_BUY
	case "sell":
		side = tradingv1.OrderSide_ORDER_SIDE_SELL
	default:
		errMsg := "invalid side: must be 'buy' or 'sell'"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	resp, err := h.tradingClient.PlaceOrder(c.Request.Context(), &tradingv1.PlaceOrderRequest{
		UserId:    user.GetId(),
		MarketId:  req.MarketID,
		OutcomeId: req.OutcomeID,
		Side:      side,
		Price:     req.Price,
		Quantity:  req.Quantity,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	order := resp.GetOrder()
	orderData := orderResponse{
		ID:             order.GetId(),
		UserID:         order.GetUserId(),
		MarketID:       order.GetMarketId(),
		OutcomeID:      order.GetOutcomeId(),
		Side:           protoOrderSideToString(order.GetSide()),
		Price:          order.GetPrice(),
		Quantity:       order.GetQuantity(),
		FilledQuantity: order.GetFilledQuantity(),
		Status:         protoOrderStatusToString(order.GetStatus()),
	}

	var trades []tradeResponse
	for _, t := range resp.GetTrades() {
		trades = append(trades, tradeResponse{
			ID:           t.GetId(),
			MakerOrderID: t.GetMakerOrderId(),
			TakerOrderID: t.GetTakerOrderId(),
			Price:        t.GetPrice(),
			Quantity:     t.GetQuantity(),
		})
	}

	result := gin.H{
		"id":              orderData.ID,
		"user_id":         orderData.UserID,
		"market_id":       orderData.MarketID,
		"outcome_id":      orderData.OutcomeID,
		"side":            orderData.Side,
		"price":           orderData.Price,
		"quantity":        orderData.Quantity,
		"filled_quantity": orderData.FilledQuantity,
		"status":          orderData.Status,
	}
	if len(trades) > 0 {
		result["trades"] = trades
	}

	data, _ := json.Marshal(result)

	c.JSON(http.StatusCreated, gin.H{
		"ok":   true,
		"data": json.RawMessage(data),
	})
}

// mintRequest is the expected JSON body for the MintTokens endpoint.
type mintRequest struct {
	Quantity string `json:"quantity"`
}

// MintTokens handles POST /api/v1/markets/:id/mint.
func (h *OrderHandler) MintTokens(c *gin.Context) {
	marketID := c.Param("id")

	var req mintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := "invalid request body"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	// Get the authenticated user from the context.
	userVal, _ := c.Get("user")
	user, ok := userVal.(*authv1.User)
	if !ok || user == nil {
		errMsg := "unauthorized"
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	resp, err := h.tradingClient.MintTokens(c.Request.Context(), &tradingv1.MintTokensRequest{
		UserId:   user.GetId(),
		MarketId: marketID,
		Quantity: req.Quantity,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	var positions []positionResponse
	for _, p := range resp.GetPositions() {
		positions = append(positions, positionResponse{
			ID:        p.GetId(),
			UserID:    p.GetUserId(),
			MarketID:  p.GetMarketId(),
			OutcomeID: p.GetOutcomeId(),
			Quantity:  p.GetQuantity(),
			AvgPrice:  p.GetAvgPrice(),
		})
	}

	result := gin.H{
		"positions": positions,
		"cost":      resp.GetCost(),
	}
	data, _ := json.Marshal(result)

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": json.RawMessage(data),
	})
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type orderResponse struct {
	ID             string `json:"id"`
	UserID         string `json:"user_id"`
	MarketID       string `json:"market_id"`
	OutcomeID      string `json:"outcome_id"`
	Side           string `json:"side"`
	Price          string `json:"price"`
	Quantity       string `json:"quantity"`
	FilledQuantity string `json:"filled_quantity"`
	Status         string `json:"status"`
}

type tradeResponse struct {
	ID           string `json:"id"`
	MakerOrderID string `json:"maker_order_id"`
	TakerOrderID string `json:"taker_order_id"`
	Price        string `json:"price"`
	Quantity     string `json:"quantity"`
}

type positionResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	MarketID  string `json:"market_id"`
	OutcomeID string `json:"outcome_id"`
	Quantity  string `json:"quantity"`
	AvgPrice  string `json:"avg_price"`
}

// protoOrderSideToString converts a proto OrderSide to a lowercase string.
func protoOrderSideToString(s tradingv1.OrderSide) string {
	switch s {
	case tradingv1.OrderSide_ORDER_SIDE_BUY:
		return "buy"
	case tradingv1.OrderSide_ORDER_SIDE_SELL:
		return "sell"
	default:
		return "unknown"
	}
}

// protoOrderStatusToString converts a proto OrderStatus to a lowercase string.
func protoOrderStatusToString(s tradingv1.OrderStatus) string {
	switch s {
	case tradingv1.OrderStatus_ORDER_STATUS_OPEN:
		return "open"
	case tradingv1.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED:
		return "partial"
	case tradingv1.OrderStatus_ORDER_STATUS_FILLED:
		return "filled"
	case tradingv1.OrderStatus_ORDER_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "unknown"
	}
}
