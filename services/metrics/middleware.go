package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HTTPMetrics returns Gin middleware that records http_requests_total and
// http_request_duration_seconds for every request. The path label uses
// Gin's matched route template (e.g. "/api/expenses/:id") to avoid
// high-cardinality label explosion from path parameters.
//
// Requests to /metrics are excluded to avoid self-referential noise from
// Prometheus scrapes.
func HTTPMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		// FullPath returns the matched route pattern, e.g. "/api/expenses/:id".
		// Falls back to "unmatched" for requests that hit no route (404s).
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}

		method := c.Request.Method

		HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}
