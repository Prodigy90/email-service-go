package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// IPWhitelist creates a middleware that restricts access to specified IPs or CIDR ranges.
// If allowedIPs is empty, all requests are allowed (dev mode).
// allowedIPs should be a comma-separated list of IPs or CIDR ranges.
// Example: "100.64.0.0/10,192.168.1.0/24,10.0.0.1"
func IPWhitelist(allowedIPs string) gin.HandlerFunc {
	// If no IPs configured, allow all (dev mode)
	if allowedIPs == "" {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Parse allowed IPs and CIDR ranges
	var allowedNets []*net.IPNet
	var allowedAddrs []net.IP

	for _, entry := range strings.Split(allowedIPs, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Try parsing as CIDR
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err == nil {
				allowedNets = append(allowedNets, ipNet)
				continue
			}
		}

		// Parse as single IP
		ip := net.ParseIP(entry)
		if ip != nil {
			allowedAddrs = append(allowedAddrs, ip)
		}
	}

	return func(c *gin.Context) {
		clientIP := net.ParseIP(c.ClientIP())
		if clientIP == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid client IP"})
			return
		}

		// Check single IPs
		for _, allowed := range allowedAddrs {
			if allowed.Equal(clientIP) {
				c.Next()
				return
			}
		}

		// Check CIDR ranges
		for _, ipNet := range allowedNets {
			if ipNet.Contains(clientIP) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied"})
	}
}
