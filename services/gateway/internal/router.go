package internal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	otelmw "github.com/truthmarket/truth-market/pkg/otel/middleware"
)

// SetupRouter creates a gin.Engine with middleware and placeholder route
// groups. Business logic handlers will be wired in later.
func SetupRouter(serviceName string) *gin.Engine {
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
		v1.Group("/auth")
		v1.Group("/markets")
		v1.Group("/trading")
		v1.Group("/rankings")
		v1.Group("/admin")
	}

	return r
}
