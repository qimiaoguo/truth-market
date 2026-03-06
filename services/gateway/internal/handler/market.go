package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
		req.Status = stringToMarketStatus(s)
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
		slog.Error("ListMarkets gRPC error", "error", err)
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
	data, _ := json.Marshal(gin.H{
		"market": mr,
	})

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
	Volume      string            `json:"volume"`
	Liquidity   string            `json:"liquidity"`
	EndTime     string            `json:"end_time,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
	ResolvedAt  string            `json:"resolved_at,omitempty"`
	Outcomes    []outcomeResponse `json:"outcomes,omitempty"`
}

type outcomeResponse struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Index    int32  `json:"index"`
	Price    string `json:"price"`
	IsWinner bool   `json:"is_winner"`
}

// stringToMarketStatus converts a simple string like "open" to the proto MarketStatus.
func stringToMarketStatus(s string) marketv1.MarketStatus {
	switch strings.ToLower(s) {
	case "draft":
		return marketv1.MarketStatus_MARKET_STATUS_DRAFT
	case "open":
		return marketv1.MarketStatus_MARKET_STATUS_OPEN
	case "closed":
		return marketv1.MarketStatus_MARKET_STATUS_CLOSED
	case "resolved":
		return marketv1.MarketStatus_MARKET_STATUS_RESOLVED
	case "cancelled":
		return marketv1.MarketStatus_MARKET_STATUS_CANCELLED
	default:
		return marketv1.MarketStatus_MARKET_STATUS_UNSPECIFIED
	}
}

// marketStatusToString converts a proto MarketStatus to a simple lowercase string.
func marketStatusToString(s marketv1.MarketStatus) string {
	switch s {
	case marketv1.MarketStatus_MARKET_STATUS_DRAFT:
		return "draft"
	case marketv1.MarketStatus_MARKET_STATUS_OPEN:
		return "open"
	case marketv1.MarketStatus_MARKET_STATUS_CLOSED:
		return "closed"
	case marketv1.MarketStatus_MARKET_STATUS_RESOLVED:
		return "resolved"
	case marketv1.MarketStatus_MARKET_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "unknown"
	}
}

// marketTypeToString converts a proto MarketType to a simple lowercase string.
func marketTypeToString(t marketv1.MarketType) string {
	switch t {
	case marketv1.MarketType_MARKET_TYPE_BINARY:
		return "binary"
	case marketv1.MarketType_MARKET_TYPE_MULTI:
		return "multi"
	default:
		return "unknown"
	}
}

// protoMarketToResponse converts a proto Market to the JSON-friendly response.
func protoMarketToResponse(m *marketv1.Market) marketResponse {
	mr := marketResponse{
		ID:          m.GetId(),
		Title:       m.GetTitle(),
		Description: m.GetDescription(),
		Category:    m.GetCategory(),
		Status:      marketStatusToString(m.GetStatus()),
		MarketType:  marketTypeToString(m.GetMarketType()),
		CreatedBy:   m.GetCreatedBy(),
		Volume:      "0",
		Liquidity:   "0",
	}
	if m.GetEndTime() != nil {
		mr.EndTime = m.GetEndTime().AsTime().Format("2006-01-02T15:04:05Z07:00")
	}
	if m.GetCreatedAt() != nil {
		mr.CreatedAt = m.GetCreatedAt().AsTime().Format("2006-01-02T15:04:05Z07:00")
	}
	if m.GetResolvedAt() != nil {
		mr.ResolvedAt = m.GetResolvedAt().AsTime().Format("2006-01-02T15:04:05Z07:00")
	}
	for _, o := range m.GetOutcomes() {
		mr.Outcomes = append(mr.Outcomes, outcomeResponse{
			ID:       o.GetId(),
			Label:    o.GetLabel(),
			Index:    o.GetIndex(),
			Price:    "0.50",
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
