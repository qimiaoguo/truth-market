package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MarketHandler handles HTTP requests for public market endpoints and
// delegates to the market-svc via gRPC.
type MarketHandler struct {
	marketClient marketv1.MarketServiceClient
}

// NewMarketHandler creates a new MarketHandler with the given gRPC market client.
func NewMarketHandler(marketClient marketv1.MarketServiceClient) *MarketHandler {
	return &MarketHandler{marketClient: marketClient}
}

// ListMarkets handles GET /api/v1/markets.
func (h *MarketHandler) ListMarkets(c *gin.Context) {
	req := &marketv1.ListMarketsRequest{}

	// Parse optional query parameters.
	if s := c.Query("status"); s != "" {
		if v, ok := marketv1.MarketStatus_value[s]; ok {
			req.Status = marketv1.MarketStatus(v)
		}
	}
	if cat := c.Query("category"); cat != "" {
		req.Category = cat
	}
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			req.Page = int32(v)
		}
	}
	if pp := c.Query("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil {
			req.PerPage = int32(v)
		}
	}
	if req.PerPage > 100 {
		req.PerPage = 100
	}

	resp, err := h.marketClient.ListMarkets(c.Request.Context(), req)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	markets := make([]marketResponse, 0, len(resp.GetMarkets()))
	for _, m := range resp.GetMarkets() {
		markets = append(markets, protoMarketToResponse(m))
	}

	data, _ := json.Marshal(gin.H{
		"markets": markets,
		"total":   resp.GetTotal(),
	})

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": json.RawMessage(data),
	})
}

// GetMarket handles GET /api/v1/markets/:id.
func (h *MarketHandler) GetMarket(c *gin.Context) {
	id := c.Param("id")

	resp, err := h.marketClient.GetMarket(c.Request.Context(), &marketv1.GetMarketRequest{
		MarketId: id,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	mr := protoMarketToResponse(resp.GetMarket())
	data, _ := json.Marshal(mr)

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": json.RawMessage(data),
	})
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

type marketResponse struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Status      string            `json:"status"`
	MarketType  string            `json:"market_type"`
	CreatedBy   string            `json:"created_by"`
	Outcomes    []outcomeResponse `json:"outcomes,omitempty"`
}

type outcomeResponse struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Index    int32  `json:"index"`
	IsWinner bool   `json:"is_winner"`
}

// protoMarketToResponse converts a proto Market to the JSON-friendly response.
func protoMarketToResponse(m *marketv1.Market) marketResponse {
	mr := marketResponse{
		ID:          m.GetId(),
		Title:       m.GetTitle(),
		Description: m.GetDescription(),
		Category:    m.GetCategory(),
		Status:      m.GetStatus().String(),
		MarketType:  m.GetMarketType().String(),
		CreatedBy:   m.GetCreatedBy(),
	}
	for _, o := range m.GetOutcomes() {
		mr.Outcomes = append(mr.Outcomes, outcomeResponse{
			ID:       o.GetId(),
			Label:    o.GetLabel(),
			Index:    o.GetIndex(),
			IsWinner: o.GetIsWinner(),
		})
	}
	return mr
}

// handleGRPCError maps gRPC status codes to HTTP status codes and returns
// a standard error response.
func handleGRPCError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		errMsg := "internal server error"
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	var httpStatus int
	var errMsg string
	switch st.Code() {
	case codes.NotFound:
		httpStatus = http.StatusNotFound
		errMsg = st.Message()
	case codes.InvalidArgument:
		httpStatus = http.StatusBadRequest
		errMsg = st.Message()
	case codes.Unauthenticated:
		httpStatus = http.StatusUnauthorized
		errMsg = st.Message()
	case codes.PermissionDenied:
		httpStatus = http.StatusForbidden
		errMsg = st.Message()
	case codes.FailedPrecondition:
		httpStatus = http.StatusConflict
		errMsg = st.Message()
	case codes.AlreadyExists:
		httpStatus = http.StatusConflict
		errMsg = st.Message()
	default:
		httpStatus = http.StatusInternalServerError
		errMsg = "internal server error"
	}

	c.JSON(httpStatus, gin.H{
		"ok":    false,
		"error": &errMsg,
	})
}
