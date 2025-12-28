package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth middleware validates API key authentication.
func APIKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check Authorization header
		auth := c.GetHeader("Authorization")
		if auth != "" {
			// Support "Bearer <key>" format
			if strings.HasPrefix(auth, "Bearer ") {
				token := strings.TrimPrefix(auth, "Bearer ")
				if token == apiKey {
					c.Next()
					return
				}
			}
		}

		// Check X-API-Key header
		key := c.GetHeader("X-API-Key")
		if key == apiKey {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized: invalid or missing API key",
		})
	}
}
