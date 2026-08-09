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

// TestNewRouter_RecoversPanicsIntoTheLogStream pins the replacement of
// gin.Recovery(), which wrote the panic as plaintext to gin.DefaultErrorWriter
// and so never reached the JSON log stream.
func TestNewRouter_RecoversPanicsIntoTheLogStream(t *testing.T) {
	logger, logs := bufferedLogger()
	withDefaultLogger(t, logger)

	router := serverkit.NewRouter("finance", false)
	router.GET("/api/boom", func(*gin.Context) { panic("handler exploded") })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/boom", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t,
		`{"code":"INTERNAL_SERVER_ERROR","message":"An unexpected error occurred"}`,
		w.Body.String(),
	)

	records := recordsAtLevel(capturedRecords(t, logs), "ERROR")
	require.Len(t, records, 1)
	assert.Equal(t, "recovered panic in HTTP handler", records[0]["msg"])
	assert.Equal(t, "/api/boom", records[0]["path"])
}
