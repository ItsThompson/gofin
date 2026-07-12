package serverkit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/serverkit"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewRouter_HealthReportsService(t *testing.T) {
	router := serverkit.NewRouter("finance", false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok","service":"finance"}`, w.Body.String())
}

func TestNewRouter_ExposesMetricsEndpoint(t *testing.T) {
	router := serverkit.NewRouter("auth", false)

	// Drive one request so the HTTP metrics middleware records an observation.
	router.GET("/api/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ping", nil))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, `path="/api/ping"`)
}

func TestNewRouter_ProductionEnablesReleaseMode(t *testing.T) {
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	serverkit.NewRouter("expense", true)

	assert.Equal(t, gin.ReleaseMode, gin.Mode())
}
