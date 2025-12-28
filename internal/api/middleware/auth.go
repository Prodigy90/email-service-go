package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth middleware validates API key authentication.
// Uses constant-time comparison to prevent timing attacks.
func APIKeyAuth(apiKey string) gin.HandlerFunc {
	apiKeyBytes := []byte(apiKey)

	return func(c *gin.Context) {
		// Check Authorization header
		auth := c.GetHeader("Authorization")
		if auth != "" {
			// Support "Bearer <key>" format
			if strings.HasPrefix(auth, "Bearer ") {
				token := strings.TrimPrefix(auth, "Bearer ")
				if subtle.ConstantTimeCompare([]byte(token), apiKeyBytes) == 1 {
					c.Next()
					return
				}
			}
		}

		// Check X-API-Key header
		key := c.GetHeader("X-API-Key")
		if subtle.ConstantTimeCompare([]byte(key), apiKeyBytes) == 1 {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized: invalid or missing API key",
		})
	}
}
