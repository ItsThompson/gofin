package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger returns Gin middleware that logs each request with structured JSON fields:
// method, path, status, duration_ms, and user_id (when authenticated).
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
		}

		// Include user_id when the auth middleware has set it in the Gin context.
		if userID, exists := c.Get("X-User-ID"); exists {
			attrs = append(attrs, slog.String("user_id", userID.(string)))
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		logger.LogAttrs(c.Request.Context(), level, "request completed", attrs...)
	}
}
