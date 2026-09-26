package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// NoStorePersonalResponses protects finance and expense routes, including
// gateway-generated authentication and proxy errors, from intermediary caches.
func NoStorePersonalResponses() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPersonalPath(c.Request.URL.Path) {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	}
}

func isPersonalPath(path string) bool {
	return path == "/api/finance" || strings.HasPrefix(path, "/api/finance/") ||
		path == "/api/expenses" || strings.HasPrefix(path, "/api/expenses/")
}
