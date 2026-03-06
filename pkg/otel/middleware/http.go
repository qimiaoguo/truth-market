// Package middleware provides OpenTelemetry-aware middleware for HTTP (Gin),
// gRPC, and domain event bus integrations used across the truth-market
// platform.
package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// HTTPTracing returns Gin middleware that creates a span for each inbound
// HTTP request. The span name is derived from the matched route and the
// serviceName is used as the instrumentation scope.
func HTTPTracing(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
