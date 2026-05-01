package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireAdmin returns Gin middleware that rejects requests where X-User-Role
// is not "admin". This middleware must run after Auth middleware, which sets
// the X-User-Role header on successful token validation.
func RequireAdmin(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.Request.Header.Get("X-User-Role")
		if role != "admin" {
			logger.Warn("admin access denied",
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.String("role", role),
				slog.String("user_id", c.Request.Header.Get("X-User-ID")),
			)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "FORBIDDEN",
				"message": "Admin access required",
			})
			return
		}

		c.Next()
	}
}
