package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
)

// AuthMiddleware returns a Gin middleware that authenticates requests via
// API key (X-API-Key header) or JWT (Authorization: Bearer header) by
// calling the auth-svc gRPC service.
//
// API key takes precedence when both headers are present.
func AuthMiddleware(authClient authv1.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check X-API-Key header first (takes precedence).
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			resp, err := authClient.ValidateAPIKey(c.Request.Context(), &authv1.ValidateAPIKeyRequest{
				ApiKey: apiKey,
			})
			if err != nil {
				errMsg := "invalid or expired API key"
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"ok":    false,
					"error": &errMsg,
				})
				return
			}
			c.Set("user", resp.User)
			c.Set("is_admin", resp.User.GetIsAdmin())
			c.Next()
			return
		}

		// Check Authorization: Bearer <token> header.
		if authHeader := c.GetHeader("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token := parts[1]
				resp, err := authClient.ValidateToken(c.Request.Context(), &authv1.ValidateTokenRequest{
					Token: token,
				})
				if err != nil {
					errMsg := "invalid or expired token"
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
						"ok":    false,
						"error": &errMsg,
					})
					return
				}
				c.Set("user", resp.User)
				c.Set("is_admin", resp.User.GetIsAdmin())
				c.Next()
				return
			}
		}

		// Neither header present.
		errMsg := "missing authentication credentials"
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"ok":    false,
			"error": &errMsg,
		})
	}
}
