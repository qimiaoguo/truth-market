package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
	"google.golang.org/grpc"
)

// mockAuthClient implements authv1.AuthServiceClient for testing.
type mockAuthClient struct {
	generateNonceResp  *authv1.GenerateNonceResponse
	generateNonceErr   error
	verifySignatureResp *authv1.VerifySignatureResponse
	verifySignatureErr  error
	validateTokenResp  *authv1.ValidateTokenResponse
	validateTokenErr   error
	validateAPIKeyResp *authv1.ValidateAPIKeyResponse
	validateAPIKeyErr  error
}

func (m *mockAuthClient) GenerateNonce(_ context.Context, _ *authv1.GenerateNonceRequest, _ ...grpc.CallOption) (*authv1.GenerateNonceResponse, error) {
	return m.generateNonceResp, m.generateNonceErr
}

func (m *mockAuthClient) VerifySignature(_ context.Context, _ *authv1.VerifySignatureRequest, _ ...grpc.CallOption) (*authv1.VerifySignatureResponse, error) {
	return m.verifySignatureResp, m.verifySignatureErr
}

func (m *mockAuthClient) ValidateToken(_ context.Context, _ *authv1.ValidateTokenRequest, _ ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	return m.validateTokenResp, m.validateTokenErr
}

func (m *mockAuthClient) ValidateAPIKey(_ context.Context, _ *authv1.ValidateAPIKeyRequest, _ ...grpc.CallOption) (*authv1.ValidateAPIKeyResponse, error) {
	return m.validateAPIKeyResp, m.validateAPIKeyErr
}

func (m *mockAuthClient) CreateAPIKey(_ context.Context, _ *authv1.CreateAPIKeyRequest, _ ...grpc.CallOption) (*authv1.CreateAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) RevokeAPIKey(_ context.Context, _ *authv1.RevokeAPIKeyRequest, _ ...grpc.CallOption) (*authv1.RevokeAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) GetUser(_ context.Context, _ *authv1.GetUserRequest, _ ...grpc.CallOption) (*authv1.GetUserResponse, error) {
	return nil, nil
}

// jsonResponse is the standard gateway API response envelope.
type jsonResponse struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error *string         `json:"error,omitempty"`
}

func setupTestRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	auth := r.Group("/api/v1/auth")
	{
		auth.GET("/nonce", h.GetNonce)
		auth.POST("/verify", h.Verify)
		auth.GET("/me", h.Me)
	}

	return r
}

func TestNonceHandler_Returns200WithNonce(t *testing.T) {
	mock := &mockAuthClient{
		generateNonceResp: &authv1.GenerateNonceResponse{
			Nonce: "random-nonce-abc123",
		},
	}
	h := &AuthHandler{authClient: mock}
	router := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/nonce", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	// Parse the data field to verify nonce is present.
	var data struct {
		Nonce string `json:"nonce"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Equal(t, "random-nonce-abc123", data.Nonce)
}

func TestVerifyHandler_ValidPayload_ReturnsJWTAndUser(t *testing.T) {
	mock := &mockAuthClient{
		verifySignatureResp: &authv1.VerifySignatureResponse{
			Token: "jwt-token-xyz",
			User: &authv1.User{
				Id:            "user-123",
				WalletAddress: "0xABC",
				UserType:      authv1.UserType_USER_TYPE_HUMAN,
				IsAdmin:       false,
			},
		},
	}
	h := &AuthHandler{authClient: mock}
	router := setupTestRouter(h)

	body := map[string]string{
		"message":        "Sign this message to verify your identity",
		"signature":      "0xsignatureabc",
		"wallet_address": "0xABC",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	// Parse the data field to verify token and user are present.
	var data struct {
		Token string `json:"token"`
		User  struct {
			ID            string `json:"id"`
			WalletAddress string `json:"wallet_address"`
		} `json:"user"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Equal(t, "jwt-token-xyz", data.Token)
	assert.Equal(t, "user-123", data.User.ID)
	assert.Equal(t, "0xABC", data.User.WalletAddress)
}

func TestVerifyHandler_InvalidPayload_Returns400(t *testing.T) {
	mock := &mockAuthClient{
		verifySignatureErr: errors.New("invalid signature"),
	}
	h := &AuthHandler{authClient: mock}
	router := setupTestRouter(h)

	// Send an invalid/empty payload.
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
}

func TestMeHandler_Authenticated_ReturnsUser(t *testing.T) {
	mock := &mockAuthClient{}
	h := &AuthHandler{authClient: mock}

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Simulate auth middleware by setting user in context.
	r.GET("/api/v1/auth/me", func(c *gin.Context) {
		c.Set("user", &authv1.User{
			Id:            "user-123",
			WalletAddress: "0xABC",
			UserType:      authv1.UserType_USER_TYPE_HUMAN,
			IsAdmin:       true,
		})
		c.Next()
	}, h.Me)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)

	// Parse data to verify user fields.
	var data struct {
		ID            string `json:"id"`
		WalletAddress string `json:"wallet_address"`
		IsAdmin       bool   `json:"is_admin"`
	}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	assert.Equal(t, "user-123", data.ID)
	assert.Equal(t, "0xABC", data.WalletAddress)
	assert.True(t, data.IsAdmin)
}

func TestMeHandler_Unauthenticated_Returns401(t *testing.T) {
	mock := &mockAuthClient{}
	h := &AuthHandler{authClient: mock}
	router := setupTestRouter(h)

	// No user set in context (no auth middleware applied).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
}
