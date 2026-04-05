package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS middleware handles Cross-Origin Resource Sharing using an explicit allowlist.
// Pass an empty slice to disable CORS (same-origin only), which is the correct default
// for server-to-server API traffic gated by API keys.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowSet[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowSet[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
				c.Header("Access-Control-Max-Age", "86400")
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
