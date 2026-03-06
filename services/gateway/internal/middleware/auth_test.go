package middleware

import (
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
	validateTokenResp  *authv1.ValidateTokenResponse
	validateTokenErr   error
	validateAPIKeyResp *authv1.ValidateAPIKeyResponse
	validateAPIKeyErr  error

	// Track which methods were called for assertions.
	validateTokenCalled  bool
	validateAPIKeyCalled bool
}

func (m *mockAuthClient) GenerateNonce(_ context.Context, _ *authv1.GenerateNonceRequest, _ ...grpc.CallOption) (*authv1.GenerateNonceResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) VerifySignature(_ context.Context, _ *authv1.VerifySignatureRequest, _ ...grpc.CallOption) (*authv1.VerifySignatureResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ValidateToken(_ context.Context, _ *authv1.ValidateTokenRequest, _ ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	m.validateTokenCalled = true
	return m.validateTokenResp, m.validateTokenErr
}

func (m *mockAuthClient) ValidateAPIKey(_ context.Context, _ *authv1.ValidateAPIKeyRequest, _ ...grpc.CallOption) (*authv1.ValidateAPIKeyResponse, error) {
	m.validateAPIKeyCalled = true
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

func setupTestRouter(mock *mockAuthClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(mock))
	r.GET("/protected", func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "user not in context"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": user})
	})
	return r
}

func TestAuthMiddleware_ValidJWT_SetsUserContext(t *testing.T) {
	mock := &mockAuthClient{
		validateTokenResp: &authv1.ValidateTokenResponse{
			User: &authv1.User{
				Id:            "user-123",
				WalletAddress: "0xABC",
				UserType:      authv1.UserType_USER_TYPE_HUMAN,
				IsAdmin:       false,
			},
		},
	}

	router := setupTestRouter(mock)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-jwt-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.True(t, mock.validateTokenCalled, "ValidateToken should have been called")
}

func TestAuthMiddleware_ValidAPIKey_SetsUserContext(t *testing.T) {
	mock := &mockAuthClient{
		validateAPIKeyResp: &authv1.ValidateAPIKeyResponse{
			User: &authv1.User{
				Id:            "user-456",
				WalletAddress: "0xDEF",
				UserType:      authv1.UserType_USER_TYPE_AGENT,
				IsAdmin:       false,
			},
		},
	}

	router := setupTestRouter(mock)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-Key", "tm_validapikey123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.True(t, mock.validateAPIKeyCalled, "ValidateAPIKey should have been called")
}

func TestAuthMiddleware_ExpiredJWT_Returns401(t *testing.T) {
	mock := &mockAuthClient{
		validateTokenErr: errors.New("token expired"),
	}

	router := setupTestRouter(mock)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer expired-jwt-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
}

func TestAuthMiddleware_InvalidAPIKey_Returns401(t *testing.T) {
	mock := &mockAuthClient{
		validateAPIKeyErr: errors.New("invalid api key"),
	}

	router := setupTestRouter(mock)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-Key", "tm_invalidkey999")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
}

func TestAuthMiddleware_NoCredentials_Returns401(t *testing.T) {
	mock := &mockAuthClient{}

	router := setupTestRouter(mock)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	// No Authorization header, no X-API-Key header.
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp jsonResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Error)
	assert.False(t, mock.validateTokenCalled, "ValidateToken should not have been called")
	assert.False(t, mock.validateAPIKeyCalled, "ValidateAPIKey should not have been called")
}

func TestAuthMiddleware_APIKeyTakesPrecedence(t *testing.T) {
	// When both X-API-Key and Authorization headers are present,
	// the middleware should check the API key first and skip JWT validation.
	mock := &mockAuthClient{
		validateAPIKeyResp: &authv1.ValidateAPIKeyResponse{
			User: &authv1.User{
				Id:            "user-789",
				WalletAddress: "0x789",
				UserType:      authv1.UserType_USER_TYPE_AGENT,
				IsAdmin:       true,
			},
		},
		validateTokenResp: &authv1.ValidateTokenResponse{
			User: &authv1.User{
				Id:            "user-other",
				WalletAddress: "0xOTHER",
				UserType:      authv1.UserType_USER_TYPE_HUMAN,
				IsAdmin:       false,
			},
		},
	}

	router := setupTestRouter(mock)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-Key", "tm_validapikey123")
	req.Header.Set("Authorization", "Bearer valid-jwt-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, mock.validateAPIKeyCalled, "ValidateAPIKey should have been called")
	assert.False(t, mock.validateTokenCalled, "ValidateToken should NOT have been called when API key is present")
}
