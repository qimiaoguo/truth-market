package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AdminHandler handles HTTP requests for admin-only market management
// endpoints and delegates to the market-svc and auth-svc via gRPC.
type AdminHandler struct {
	marketClient marketv1.MarketServiceClient
	authClient   authv1.AuthServiceClient
}

// NewAdminHandler creates a new AdminHandler with the given gRPC clients.
func NewAdminHandler(marketClient marketv1.MarketServiceClient, authClient authv1.AuthServiceClient) *AdminHandler {
	return &AdminHandler{marketClient: marketClient, authClient: authClient}
}

// requireAdmin checks if the current user is an admin. Returns false and
// writes a 403 response if not.
func requireAdmin(c *gin.Context) bool {
	isAdmin, exists := c.Get("is_admin")
	if !exists {
		errMsg := "forbidden: admin access required"
		c.JSON(http.StatusForbidden, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return false
	}
	admin, ok := isAdmin.(bool)
	if !ok || !admin {
		errMsg := "forbidden: admin access required"
		c.JSON(http.StatusForbidden, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return false
	}
	return true
}

// createMarketRequest is the expected JSON body for CreateMarket.
type createMarketRequest struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	MarketType    string   `json:"market_type"`
	Category      string   `json:"category"`
	OutcomeLabels []string `json:"outcome_labels"`
	EndTime       string   `json:"end_time,omitempty"`
}

// CreateMarket handles POST /api/v1/admin/markets.
func (h *AdminHandler) CreateMarket(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req createMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := "invalid request body"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	// Validate input lengths.
	if len(req.Title) > 256 {
		errMsg := "title must not exceed 256 characters"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}
	if len(req.Description) > 5000 {
		errMsg := "description must not exceed 5000 characters"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}
	if len(req.Category) > 100 {
		errMsg := "category must not exceed 100 characters"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	// Map string market type to proto enum.
	protoType := marketv1.MarketType_MARKET_TYPE_UNSPECIFIED
	if v, ok := marketv1.MarketType_value[req.MarketType]; ok {
		protoType = marketv1.MarketType(v)
	}

	grpcReq := &marketv1.CreateMarketRequest{
		Title:         req.Title,
		Description:   req.Description,
		MarketType:    protoType,
		Category:      req.Category,
		OutcomeLabels: req.OutcomeLabels,
		EndTime:       timestamppb.Now(), // default; in production, parse from req.EndTime
	}

	// Extract created_by from authenticated user context.
	if userVal, exists := c.Get("user"); exists {
		if u, ok := userVal.(interface{ GetId() string }); ok {
			grpcReq.CreatedBy = u.GetId()
		}
	}

	resp, err := h.marketClient.CreateMarket(c.Request.Context(), grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	mr := protoMarketToResponse(resp.GetMarket())
	data, _ := json.Marshal(mr)

	c.JSON(http.StatusCreated, gin.H{
		"ok":   true,
		"data": json.RawMessage(data),
	})
}

// resolveMarketRequest is the expected JSON body for ResolveMarket.
type resolveMarketRequest struct {
	WinningOutcomeID string `json:"winning_outcome_id"`
}

// ResolveMarket handles POST /api/v1/admin/markets/:id/resolve.
func (h *AdminHandler) ResolveMarket(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	marketID := c.Param("id")

	var req resolveMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := "invalid request body"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	resp, err := h.marketClient.ResolveMarket(c.Request.Context(), &marketv1.ResolveMarketRequest{
		MarketId:         marketID,
		WinningOutcomeId: req.WinningOutcomeID,
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

// createAgentUserRequest is the expected JSON body for CreateAgentUser.
type createAgentUserRequest struct {
	WalletAddress string `json:"wallet_address"`
}

// CreateAgentUser handles POST /api/v1/admin/agent-users.
// It creates a new agent (bot) user by calling auth-svc VerifySignature
// with a system-generated identity.
func (h *AdminHandler) CreateAgentUser(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req createAgentUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := "invalid request body"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	if req.WalletAddress == "" {
		errMsg := "wallet_address is required"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	// Use VerifySignature with an empty signature to register an agent user.
	// The auth-svc is responsible for recognising this pattern and creating
	// an agent-type user.
	resp, err := h.authClient.VerifySignature(c.Request.Context(), &authv1.VerifySignatureRequest{
		WalletAddress: req.WalletAddress,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	user := resp.GetUser()
	c.JSON(http.StatusCreated, gin.H{
		"ok": true,
		"data": gin.H{
			"id":             user.GetId(),
			"wallet_address": user.GetWalletAddress(),
			"user_type":      user.GetUserType().String(),
			"token":          resp.GetToken(),
		},
	})
}
