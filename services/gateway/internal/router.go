package internal

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/truthmarket/truth-market/services/gateway/internal/handler"
	"github.com/truthmarket/truth-market/services/gateway/internal/middleware"
	otelmw "github.com/truthmarket/truth-market/pkg/otel/middleware"
)

// SetupRouter creates a gin.Engine with middleware and all API route
// registrations. Handler instances and the auth middleware are injected
// from the caller (main.go) so the router itself stays free of gRPC
// client construction.
//
// The limiter parameter is optional: when nil, rate limiting is silently
// skipped so the gateway can start without Redis.
func SetupRouter(
	serviceName string,
	limiter middleware.RateLimiter,
	authHandler *handler.AuthHandler,
	marketHandler *handler.MarketHandler,
	orderHandler *handler.OrderHandler,
	rankingHandler *handler.RankingHandler,
	adminHandler *handler.AdminHandler,
	authMW gin.HandlerFunc,
) *gin.Engine {
	r := gin.Default()

	// CORS: allow frontend origins.
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

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

	v1 := r.Group("/api/v1")
	{
		// ----- Auth endpoints: strict rate limit (10 req/min per IP). -----
		auth := v1.Group("/auth")
		auth.Use(middleware.RateLimitMiddleware(limiter, middleware.RateLimitConfig{
			Limit:  10,
			Window: 1 * time.Minute,
		}))
		{
			auth.GET("/nonce", authHandler.GetNonce)
			auth.POST("/verify", authHandler.Verify)
			auth.GET("/me", authMW, authHandler.Me)
			auth.POST("/api-key", authMW, authHandler.CreateAPIKey)
		}

		// ----- Market endpoints: light rate limit (120 req/min per IP/user). -----
		markets := v1.Group("/markets")
		markets.Use(middleware.RateLimitMiddleware(limiter, middleware.RateLimitConfig{
			Limit:  120,
			Window: 1 * time.Minute,
		}))
		{
			markets.GET("", marketHandler.ListMarkets)
			markets.GET("/:id", marketHandler.GetMarket)
			markets.GET("/:id/orderbook", orderHandler.GetOrderbook)
		}

		// ----- Trading endpoints: medium rate limit (60 req/min per user). -----
		trading := v1.Group("/trading")
		trading.Use(middleware.RateLimitMiddleware(limiter, middleware.RateLimitConfig{
			Limit:  60,
			Window: 1 * time.Minute,
		}))
		{
			trading.POST("/mint", authMW, orderHandler.MintTokens)
			trading.POST("/orders", authMW, orderHandler.PlaceOrder)
			trading.DELETE("/orders/:id", authMW, orderHandler.CancelOrder)
			trading.GET("/orders", authMW, orderHandler.ListOrders)
			trading.GET("/positions", authMW, orderHandler.GetPositions)
		}

		// ----- Rankings endpoints: light rate limit (120 req/min per IP/user). -----
		rankings := v1.Group("/rankings")
		rankings.Use(middleware.RateLimitMiddleware(limiter, middleware.RateLimitConfig{
			Limit:  120,
			Window: 1 * time.Minute,
		}))
		{
			rankings.GET("", rankingHandler.GetLeaderboard)
			rankings.GET("/user/:id", rankingHandler.GetUserRanking)
		}

		// ----- Admin endpoints: auth + admin required, no additional rate limit. -----
		admin := v1.Group("/admin")
		admin.Use(authMW)
		{
			admin.POST("/markets", adminHandler.CreateMarket)
			admin.POST("/markets/:id/resolve", adminHandler.ResolveMarket)
			admin.POST("/agent-users", adminHandler.CreateAgentUser)
		}
	}

	return r
}
