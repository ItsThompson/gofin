package readiness_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/gateway/internal/readiness"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newHealthServer returns an httptest server whose /health responds with the
// given status code. Any other path 404s, so a probe hitting the wrong path
// surfaces as unhealthy rather than silently passing.
func newHealthServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server
}

func newChecker(services map[string]string, timeout time.Duration) *readiness.Checker {
	return readiness.NewChecker(&http.Client{}, services, timeout)
}

func TestChecker_AllHealthy_ReportsHealthy(t *testing.T) {
	auth := newHealthServer(t, http.StatusOK)
	expense := newHealthServer(t, http.StatusOK)

	checker := newChecker(map[string]string{
		"auth":    auth.URL,
		"expense": expense.URL,
	}, 2*time.Second)

	result := checker.Check(context.Background())

	assert.True(t, result.Healthy)
	assert.Equal(t, map[string]string{"auth": "ok", "expense": "ok"}, result.Services)
}

func TestChecker_OneUnhealthy_ReportsUnhealthyAndNamesService(t *testing.T) {
	auth := newHealthServer(t, http.StatusOK)
	expense := newHealthServer(t, http.StatusServiceUnavailable)

	checker := newChecker(map[string]string{
		"auth":    auth.URL,
		"expense": expense.URL,
	}, 2*time.Second)

	result := checker.Check(context.Background())

	assert.False(t, result.Healthy)
	assert.Equal(t, "ok", result.Services["auth"])
	assert.Equal(t, "status_503", result.Services["expense"], "the failing service must be named with its status")
}

func TestChecker_Unreachable_ReportsUnreachable(t *testing.T) {
	auth := newHealthServer(t, http.StatusOK)
	// A closed server's address is unreachable: the probe must classify it as
	// such rather than hang or panic.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	deadURL := dead.URL
	dead.Close()

	checker := newChecker(map[string]string{
		"auth":    auth.URL,
		"expense": deadURL,
	}, 2*time.Second)

	result := checker.Check(context.Background())

	assert.False(t, result.Healthy)
	assert.Equal(t, "ok", result.Services["auth"])
	assert.Equal(t, "unreachable", result.Services["expense"])
}

func TestChecker_Timeout_ReportsUnreachableWithinBound(t *testing.T) {
	auth := newHealthServer(t, http.StatusOK)
	// A downstream whose /health hangs past the probe timeout must be bounded:
	// Check returns within the timeout (plus slack) and marks it unreachable.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(slow.Close)

	checker := newChecker(map[string]string{
		"auth":    auth.URL,
		"expense": slow.URL,
	}, 100*time.Millisecond)

	start := time.Now()
	result := checker.Check(context.Background())
	elapsed := time.Since(start)

	assert.False(t, result.Healthy)
	assert.Equal(t, "unreachable", result.Services["expense"])
	assert.Less(t, elapsed, time.Second, "a hung downstream must not block past the per-probe timeout")
}

// TestChecker_ProbesConcurrently proves the fan-out is concurrent: several slow
// downstreams settle in roughly one timeout window, not the sum of them.
func TestChecker_ProbesConcurrently(t *testing.T) {
	var inFlight, maxInFlight int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			observed := atomic.LoadInt32(&maxInFlight)
			if current <= observed || atomic.CompareAndSwapInt32(&maxInFlight, observed, current) {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.WriteHeader(http.StatusOK)
	})

	services := map[string]string{}
	for _, name := range []string{"auth", "expense", "finance", "datarights"} {
		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)
		services[name] = server.URL
	}

	checker := newChecker(services, 2*time.Second)

	start := time.Now()
	result := checker.Check(context.Background())
	elapsed := time.Since(start)

	require.True(t, result.Healthy)
	assert.Less(t, elapsed, 500*time.Millisecond, "four 150ms probes run serially would exceed 500ms")
	assert.Equal(t, int32(4), atomic.LoadInt32(&maxInFlight), "all four probes should be in flight at once")
}

// serveReadyz wires readiness.Handler behind a gin engine and returns the
// recorded response for GET /readyz.
func serveReadyz(checker *readiness.Checker) *httptest.ResponseRecorder {
	engine := gin.New()
	engine.GET("/readyz", readiness.Handler(checker, silentLogger()))
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestHandler_AllHealthy_Returns200OK(t *testing.T) {
	auth := newHealthServer(t, http.StatusOK)
	expense := newHealthServer(t, http.StatusOK)

	checker := newChecker(map[string]string{
		"auth":    auth.URL,
		"expense": expense.URL,
	}, 2*time.Second)

	rec := serveReadyz(checker)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestHandler_OneDown_Returns503NamingService(t *testing.T) {
	auth := newHealthServer(t, http.StatusOK)
	expense := newHealthServer(t, http.StatusServiceUnavailable)

	checker := newChecker(map[string]string{
		"auth":    auth.URL,
		"expense": expense.URL,
	}, 2*time.Second)

	rec := serveReadyz(checker)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t,
		`{"status":"unhealthy","services":{"auth":"ok","expense":"status_503"}}`,
		rec.Body.String(),
	)
}
