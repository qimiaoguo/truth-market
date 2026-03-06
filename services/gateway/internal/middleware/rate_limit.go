package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
)

// RateLimiter abstracts a sliding-window rate limiter so the middleware is not
// coupled to a concrete implementation (e.g. Redis). The infra/redis
// RateLimiter already satisfies this interface.
type RateLimiter interface {
	// Allow checks whether a request identified by key is within the rate
	// limit. It returns true when the request should be allowed.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// RateLimitConfig holds the per-route-group rate limit parameters.
type RateLimitConfig struct {
	// Limit is the maximum number of requests allowed within the Window.
	Limit int
	// Window is the time window during which Limit requests are allowed.
	Window time.Duration
}

// RateLimitMiddleware returns a Gin middleware that enforces rate limiting
// using the provided RateLimiter.
//
// The rate limit key is derived from the authenticated user ID when available
// (set by AuthMiddleware as "user" in the Gin context) and falls back to the
// client IP address for unauthenticated requests.
//
// If limiter is nil the middleware becomes a no-op, allowing the gateway to
// start even when Redis is unavailable.
func RateLimitMiddleware(limiter RateLimiter, cfg RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Gracefully skip rate limiting when no limiter is configured.
		if limiter == nil {
			c.Next()
			return
		}

		key := rateLimitKey(c)

		allowed, err := limiter.Allow(c.Request.Context(), key, cfg.Limit, cfg.Window)
		if err != nil {
			// On error, log and allow the request through so a transient
			// Redis failure does not block traffic.
			slog.Error("rate limiter error", "error", err, "key", key)
			c.Next()
			return
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "RATE_LIMITED",
					"message": "too many requests",
				},
			})
			return
		}

		c.Next()
	}
}

// rateLimitKey returns a rate-limit key for the current request.
// Authenticated requests are keyed by "user:<id>", unauthenticated ones
// by "ip:<client_ip>".
func rateLimitKey(c *gin.Context) string {
	if userVal, exists := c.Get("user"); exists {
		if user, ok := userVal.(*authv1.User); ok && user != nil {
			return fmt.Sprintf("user:%s", user.GetId())
		}
	}
	return fmt.Sprintf("ip:%s", c.ClientIP())
}
