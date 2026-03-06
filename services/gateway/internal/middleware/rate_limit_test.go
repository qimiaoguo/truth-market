package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
)

// mockRateLimiter implements RateLimiter for testing.
type mockRateLimiter struct {
	allowed  bool
	err      error
	lastKey  string
	lastLimit int
}

func (m *mockRateLimiter) Allow(_ context.Context, key string, limit int, _ time.Duration) (bool, error) {
	m.lastKey = key
	m.lastLimit = limit
	return m.allowed, m.err
}

// rateLimitResponse matches the JSON returned by the rate limit middleware.
type rateLimitResponse struct {
	OK    bool `json:"ok"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func setupRateLimitRouter(limiter RateLimiter, cfg RateLimitConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimitMiddleware(limiter, cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestRateLimitMiddleware_AllowedRequest(t *testing.T) {
	mock := &mockRateLimiter{allowed: true}
	router := setupRateLimitRouter(mock, RateLimitConfig{Limit: 10, Window: time.Minute})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, true, resp["ok"])
}

func TestRateLimitMiddleware_RateLimited(t *testing.T) {
	mock := &mockRateLimiter{allowed: false}
	router := setupRateLimitRouter(mock, RateLimitConfig{Limit: 10, Window: time.Minute})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var resp rateLimitResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "RATE_LIMITED", resp.Error.Code)
	assert.Equal(t, "too many requests", resp.Error.Message)
}

func TestRateLimitMiddleware_IPKeyForUnauthenticated(t *testing.T) {
	mock := &mockRateLimiter{allowed: true}
	router := setupRateLimitRouter(mock, RateLimitConfig{Limit: 10, Window: time.Minute})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ip:192.168.1.100", mock.lastKey)
}

func TestRateLimitMiddleware_UserIDKeyForAuthenticated(t *testing.T) {
	mock := &mockRateLimiter{allowed: true}

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Simulate auth middleware setting user in context.
	r.Use(func(c *gin.Context) {
		c.Set("user", &authv1.User{
			Id:            "user-abc-123",
			WalletAddress: "0xDEAD",
		})
		c.Next()
	})
	r.Use(RateLimitMiddleware(mock, RateLimitConfig{Limit: 60, Window: time.Minute}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user:user-abc-123", mock.lastKey)
	assert.Equal(t, 60, mock.lastLimit)
}

func TestRateLimitMiddleware_NilLimiter_AllowsThrough(t *testing.T) {
	router := setupRateLimitRouter(nil, RateLimitConfig{Limit: 10, Window: time.Minute})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitMiddleware_LimiterError_AllowsThrough(t *testing.T) {
	mock := &mockRateLimiter{
		allowed: false,
		err:     errors.New("redis connection refused"),
	}
	router := setupRateLimitRouter(mock, RateLimitConfig{Limit: 10, Window: time.Minute})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// On error the middleware should fail open (allow the request).
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitMiddleware_ConfigPassedToLimiter(t *testing.T) {
	mock := &mockRateLimiter{allowed: true}
	cfg := RateLimitConfig{Limit: 42, Window: 5 * time.Minute}
	router := setupRateLimitRouter(mock, cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 42, mock.lastLimit)
}
