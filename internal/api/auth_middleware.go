package api

import (
	"crypto/sha256"
	"crypto/subtle"
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

		if !tokensEqual(strings.TrimPrefix(auth, "Bearer "), token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}

// RequireTokenMiddleware rejects all requests when no API token is
// configured, so endpoints that change state are never open to anyone who can
// reach the port. Use it together with BearerAuthMiddleware.
func RequireTokenMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "write endpoints are disabled, set server.api_token to enable them"})
			return
		}
		c.Next()
	}
}

// tokensEqual compares two tokens in constant time. Hashing first makes the
// comparison independent of the token lengths as well.
func tokensEqual(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}
