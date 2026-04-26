package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BearerAuthMiddleware returns a Gin middleware that validates Bearer token authentication.
// If token is empty, authentication is disabled and all requests pass through.
func BearerAuthMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if strings.TrimPrefix(auth, "Bearer ") != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}
