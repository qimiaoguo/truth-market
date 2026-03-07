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
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestContract_GetNonce verifies every JSON field in the nonce response.
func TestContract_GetNonce(t *testing.T) {
	mock := &mockAuthClient{
		generateNonceResp: &authv1.GenerateNonceResponse{
			Nonce: "test-nonce-abc123",
		},
	}
	h := NewAuthHandler(mock)
	router := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/nonce", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var data struct {
		Nonce string `json:"nonce"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Equal(t, "test-nonce-abc123", data.Nonce, "nonce field missing or wrong")

	// Verify raw JSON keys.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "nonce", "top-level 'nonce' key missing")
}

// TestContract_VerifySignature verifies every JSON field in the verify response.
func TestContract_VerifySignature(t *testing.T) {
	now := timestamppb.Now()
	mock := &mockAuthClient{
		verifySignatureResp: &authv1.VerifySignatureResponse{
			Token: "jwt-token-xyz789",
			User: &authv1.User{
				Id:            "user-42",
				WalletAddress: "0x1234567890ABCDEF",
				UserType:      authv1.UserType_USER_TYPE_HUMAN,
				Balance:       "1000.00",
				LockedBalance: "50.00",
				IsAdmin:       true,
				CreatedAt:     now,
			},
		},
	}
	h := NewAuthHandler(mock)
	router := setupTestRouter(h)

	body, _ := json.Marshal(map[string]string{
		"message":        "Sign this message",
		"signature":      "0xsig123",
		"wallet_address": "0x1234567890ABCDEF",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	var data struct {
		Token string `json:"token"`
		User  struct {
			ID            string `json:"id"`
			WalletAddress string `json:"wallet_address"`
			UserType      string `json:"user_type"`
			Balance       string `json:"balance"`
			LockedBalance string `json:"locked_balance"`
			IsAdmin       bool   `json:"is_admin"`
			CreatedAt     string `json:"created_at"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	assert.Equal(t, "jwt-token-xyz789", data.Token, "token field missing or wrong")
	assert.Equal(t, "user-42", data.User.ID, "user.id field missing or wrong")
	assert.Equal(t, "0x1234567890ABCDEF", data.User.WalletAddress, "user.wallet_address field missing or wrong")
	assert.Equal(t, "human", data.User.UserType, "user.user_type field missing or wrong")
	assert.Equal(t, "1000.00", data.User.Balance, "user.balance field missing or wrong")
	assert.Equal(t, "50.00", data.User.LockedBalance, "user.locked_balance field missing or wrong")
	assert.True(t, data.User.IsAdmin, "user.is_admin field missing or wrong")
	assert.NotEmpty(t, data.User.CreatedAt, "user.created_at field missing")

	// Verify top-level keys.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(resp.Data, &raw))
	assert.Contains(t, raw, "token")
	assert.Contains(t, raw, "user")
}

// TestContract_Me verifies every JSON field in the /me response.
func TestContract_Me(t *testing.T) {
	now := timestamppb.Now()
	mock := &mockAuthClient{}
	h := NewAuthHandler(mock)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/auth/me", func(c *gin.Context) {
		c.Set("user", &authv1.User{
			Id:            "user-42",
			WalletAddress: "0x1234567890ABCDEF",
			UserType:      authv1.UserType_USER_TYPE_AGENT,
			Balance:       "500.00",
			LockedBalance: "25.00",
			IsAdmin:       false,
			CreatedAt:     now,
		})
		c.Next()
	}, h.Me)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)

	// Me returns user data directly under "data" (not wrapped in {user: ...}).
	var data struct {
		ID            string `json:"id"`
		WalletAddress string `json:"wallet_address"`
		UserType      string `json:"user_type"`
		Balance       string `json:"balance"`
		LockedBalance string `json:"locked_balance"`
		IsAdmin       bool   `json:"is_admin"`
		CreatedAt     string `json:"created_at"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &data))

	assert.Equal(t, "user-42", data.ID, "id field missing or wrong")
	assert.Equal(t, "0x1234567890ABCDEF", data.WalletAddress, "wallet_address field missing or wrong")
	assert.Equal(t, "agent", data.UserType, "user_type field missing or wrong")
	assert.Equal(t, "500.00", data.Balance, "balance field missing or wrong")
	assert.Equal(t, "25.00", data.LockedBalance, "locked_balance field missing or wrong")
	assert.False(t, data.IsAdmin, "is_admin field wrong")
	assert.NotEmpty(t, data.CreatedAt, "created_at field missing")
}
