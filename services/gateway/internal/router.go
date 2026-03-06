package internal

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truthmarket/truth-market/services/gateway/internal/middleware"
	otelmw "github.com/truthmarket/truth-market/pkg/otel/middleware"
)

// SetupRouter creates a gin.Engine with middleware and placeholder route
// groups. Business logic handlers will be wired in later.
//
// The limiter parameter is optional: when nil, rate limiting is silently
// skipped so the gateway can start without Redis.
func SetupRouter(serviceName string, limiter middleware.RateLimiter) *gin.Engine {
	r := gin.Default()

	// Limit request body size to 1MB.
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20) // 1MB
		c.Next()
	})

	// OpenTelemetry HTTP tracing middleware.
	r.Use(otelmw.HTTPTracing(serviceName))

	// Health check.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Placeholder route groups -- handlers will be registered once the
	// gRPC-backed handler layer is implemented.
	v1 := r.Group("/api/v1")
	{
		// Auth endpoints: strict rate limit (10 req/min per IP).
		auth := v1.Group("/auth")
		auth.Use(middleware.RateLimitMiddleware(limiter, middleware.RateLimitConfig{
			Limit:  10,
			Window: 1 * time.Minute,
		}))

		// Market endpoints: light rate limit (120 req/min per IP/user).
		markets := v1.Group("/markets")
		markets.Use(middleware.RateLimitMiddleware(limiter, middleware.RateLimitConfig{
			Limit:  120,
			Window: 1 * time.Minute,
		}))

		// Trading endpoints: medium rate limit (60 req/min per user).
		trading := v1.Group("/trading")
		trading.Use(middleware.RateLimitMiddleware(limiter, middleware.RateLimitConfig{
			Limit:  60,
			Window: 1 * time.Minute,
		}))

		// Rankings endpoints: light rate limit (120 req/min per IP/user).
		rankings := v1.Group("/rankings")
		rankings.Use(middleware.RateLimitMiddleware(limiter, middleware.RateLimitConfig{
			Limit:  120,
			Window: 1 * time.Minute,
		}))

		// Admin endpoints: no additional rate limit beyond the global ones.
		v1.Group("/admin")

		// Suppress unused variable warnings while route groups are
		// still placeholders.
		_ = auth
		_ = markets
		_ = trading
		_ = rankings
	}

	return r
}
