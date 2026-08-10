package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/metrics"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMetricsEndpoint_ReturnsPrometheusFormat(t *testing.T) {
	router := gin.New()
	router.Use(metrics.HTTPMetrics())
	metrics.Register(router)

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Trigger observations so vector metrics appear in output.
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	metrics.GRPCRequestsTotal.WithLabelValues("/test.Service/Method", "OK").Inc()
	metrics.GRPCRequestDuration.WithLabelValues("/test.Service/Method").Observe(0.01)
	metrics.TokenRefreshTotal.WithLabelValues("success").Inc()
	metrics.RecoveredPanicsTotal.WithLabelValues("http").Inc()

	// Now scrape /metrics.
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "# HELP http_requests_total")
	assert.Contains(t, body, "# TYPE http_requests_total counter")
	assert.Contains(t, body, "# HELP http_request_duration_seconds")
	assert.Contains(t, body, "# TYPE http_request_duration_seconds histogram")
	assert.Contains(t, body, "# HELP grpc_requests_total")
	assert.Contains(t, body, "# HELP grpc_request_duration_seconds")
	assert.Contains(t, body, "# HELP expense_entries_total")
	assert.Contains(t, body, "# HELP corrections_total")
	assert.Contains(t, body, "# HELP token_refresh_total")
	assert.Contains(t, body, "# HELP recovered_panics_total")
	assert.Contains(t, body, "# TYPE recovered_panics_total counter")
	assert.Contains(t, body, `recovered_panics_total{site="http"}`)
	// active_connections is not a defined metric and must never be exported.
	assert.NotContains(t, body, "active_connections")
}

func TestHTTPMetrics_RecordsRequestMetrics(t *testing.T) {
	router := gin.New()
	router.Use(metrics.HTTPMetrics())
	metrics.Register(router)

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Make a request to trigger metric recording.
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Now scrape /metrics and verify the request was recorded.
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `http_requests_total{method="GET",path="/api/test",status="200"}`)
	assert.Contains(t, body, `http_request_duration_seconds_bucket{method="GET",path="/api/test"`)
}

func TestHTTPMetrics_SkipsMetricsEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(metrics.HTTPMetrics())
	metrics.Register(router)

	// Scrape /metrics twice: should not produce http_requests_total for /metrics.
	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	// The /metrics path should NOT appear in http_requests_total labels.
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "http_requests_total") {
			assert.NotContains(t, line, `path="/metrics"`)
		}
	}
}

func TestHTTPMetrics_UsesRouteTemplate(t *testing.T) {
	router := gin.New()
	router.Use(metrics.HTTPMetrics())
	metrics.Register(router)

	router.GET("/api/items/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/items/abc-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	// Should use the route template, not the actual path with the UUID.
	assert.Contains(t, body, `path="/api/items/:id"`)
	assert.NotContains(t, body, `path="/api/items/abc-123"`)
}

func TestCustomMetrics_IncrementAndAppear(t *testing.T) {
	router := gin.New()
	metrics.Register(router)

	metrics.ExpenseEntriesTotal.Inc()
	metrics.CorrectionsTotal.Inc()
	metrics.TokenRefreshTotal.WithLabelValues("success").Inc()
	metrics.TokenRefreshTotal.WithLabelValues("failure").Inc()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "expense_entries_total")
	assert.Contains(t, body, "corrections_total")
	assert.Contains(t, body, `token_refresh_total{status="success"}`)
	assert.Contains(t, body, `token_refresh_total{status="failure"}`)
}

