package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/gateway/internal/middleware"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

// One failing request must cost one event, and this is not the component that owns
// it. Three components see a request-scoped 500: the service that holds the error
// value and its stack, apierr.Respond, and this logger, which sees a status code
// and nothing else. A report from here could carry no stack and no group key worth
// having, and it would bill a second event for a failure the service already
// reported.
//
// It keeps logging at error level, so the metrics-and-logs view is unchanged. That
// half is asserted by TestRequestLogger_LogsErrorFor5xx; this is the other half.
func TestRequestLogger_A5xxReportsNothing(t *testing.T) {
	logger, sink := serverkittest.NewLogger()

	router := gin.New()
	router.Use(middleware.RequestLogger(logger))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	transport := &errkittest.Transport{}
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req.WithContext(errkittest.ContextWithHub(req.Context(), transport)))

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	records, err := sink.ErrorRecords()
	require.NoError(t, err)
	require.Len(t, records, 1, "the error-level record stays")

	assert.Empty(t, transport.Events(),
		"the service that holds the error value is the only owner of a 500")
}

// A 4xx guard logs at warn, and nothing below error level ever reports: an expired
// session and an unauthenticated bot probe both reach these paths, and the public
// route is behind a Cloudflare tunnel.
func TestRequestLogger_A4xxReportsNothing(t *testing.T) {
	logger, sink := serverkittest.NewLogger()

	router := gin.New()
	router.Use(middleware.RequestLogger(logger))
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	transport := &errkittest.Transport{}
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	router.ServeHTTP(httptest.NewRecorder(), req.WithContext(errkittest.ContextWithHub(req.Context(), transport)))

	warns, err := sink.RecordsAtLevel("WARN")
	require.NoError(t, err)
	require.Len(t, warns, 1)

	assert.Empty(t, transport.Events())
}
