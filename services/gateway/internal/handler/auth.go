package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
)

// AuthHandler handles HTTP requests for authentication endpoints and
// delegates to the auth-svc via gRPC.
type AuthHandler struct {
	authClient authv1.AuthServiceClient
}

// NewAuthHandler creates a new AuthHandler with the given gRPC auth client.
func NewAuthHandler(authClient authv1.AuthServiceClient) *AuthHandler {
	return &AuthHandler{authClient: authClient}
}

// GetNonce handles GET /api/v1/auth/nonce.
// It calls auth-svc GenerateNonce and returns the nonce.
func (h *AuthHandler) GetNonce(c *gin.Context) {
	resp, err := h.authClient.GenerateNonce(c.Request.Context(), &authv1.GenerateNonceRequest{})
	if err != nil {
		errMsg := "internal server error"
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"data": gin.H{
			"nonce": resp.GetNonce(),
		},
	})
}

// verifyRequest is the expected JSON body for the Verify endpoint.
type verifyRequest struct {
	Message       string `json:"message"`
	Signature     string `json:"signature"`
	WalletAddress string `json:"wallet_address"`
}

// userResponse is the JSON representation of a user in API responses.
type userResponse struct {
	ID            string `json:"id"`
	WalletAddress string `json:"wallet_address"`
	IsAdmin       bool   `json:"is_admin"`
}

// Verify handles POST /api/v1/auth/verify.
// It calls auth-svc VerifySignature with the wallet signature and returns
// a JWT token and user info.
func (h *AuthHandler) Verify(c *gin.Context) {
	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := "invalid request body"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	// Validate required fields.
	if req.Message == "" || req.Signature == "" || req.WalletAddress == "" {
		errMsg := "message, signature, and wallet_address are required"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	// Validate input lengths.
	if len(req.WalletAddress) > 42 {
		errMsg := "wallet_address must not exceed 42 characters"
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	resp, err := h.authClient.VerifySignature(c.Request.Context(), &authv1.VerifySignatureRequest{
		Message:       req.Message,
		Signature:     req.Signature,
		WalletAddress: req.WalletAddress,
	})
	if err != nil {
		errMsg := "authentication failed"
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	user := resp.GetUser()
	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"data": gin.H{
			"token": resp.GetToken(),
			"user": userResponse{
				ID:            user.GetId(),
				WalletAddress: user.GetWalletAddress(),
				IsAdmin:       user.GetIsAdmin(),
			},
		},
	})
}

// Me handles GET /api/v1/auth/me.
// It returns the current authenticated user from the Gin context (set by
// the auth middleware).
func (h *AuthHandler) Me(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		errMsg := "unauthorized"
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	user, ok := userVal.(*authv1.User)
	if !ok || user == nil {
		errMsg := "unauthorized"
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"data": userResponse{
			ID:            user.GetId(),
			WalletAddress: user.GetWalletAddress(),
			IsAdmin:       user.GetIsAdmin(),
		},
	})
}
