package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedaccess "github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/errkit/errkittest"
	"github.com/ItsThompson/gofin/services/gateway/internal/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/router"
	"github.com/ItsThompson/gofin/services/serverkit/serverkittest"
)

// The gateway builds its own engine instead of using serverkit.NewRouter, so its
// middleware order is asserted here rather than inherited. These are the same
// properties serverkit's router carries, restated because the wiring is separate.

// hubObservingValidator records whether a Sentry hub was on the request context
// by the time the request reached an injected collaborator. Token validation is
// the first place gateway code runs on a non-public request, so a hub there means
// every downstream reporter finds one.
type hubObservingValidator struct {
	sawHub bool
	result *access.TokenValidationResult
}

func (v *hubObservingValidator) ValidateToken(ctx context.Context, _ string) (*access.TokenValidationResult, error) {
	v.sawHub = sentry.GetHubFromContext(ctx) != nil
	return v.result, nil
}

// bindRecordingHub binds a recording client to the process-wide hub, which is
// where sentrygin derives its per-request hub from, and restores the previous
// client afterwards. Tests using it must not run in parallel.
func bindRecordingHub(t *testing.T) *errkittest.Transport {
	t.Helper()

	transport := &errkittest.Transport{}
	previous := sentry.CurrentHub().Client()
	t.Cleanup(func() { sentry.CurrentHub().BindClient(previous) })
	sentry.CurrentHub().BindClient(errkittest.NewClient(transport))

	return transport
}

func newGatewayEngine(t *testing.T, validator access.TokenValidator) http.Handler {
	t.Helper()

	target, err := url.Parse("http://127.0.0.1:1")
	require.NoError(t, err)

	return router.New(validator, &router.ServiceURLs{
		AuthREST:       target,
		ExpenseREST:    target,
		FinanceREST:    target,
		DatarightsREST: target,
	}, sharedaccess.Prefixes(), newSilentLogger(), false)
}

func TestRouter_PutsAHubOnTheRequestContext(t *testing.T) {
	validator := &hubObservingValidator{result: &access.TokenValidationResult{
		UserID: "user-1", Role: "user",
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(validCookie())
	newGatewayEngine(t, validator).ServeHTTP(httptest.NewRecorder(), req)

	assert.True(t, validator.sawHub, "sentrygin must put a hub on the request context")
}

// TestRouter_APanickingRequestYieldsExactlyOneEvent is the assertion that catches
// the recovery re-panicking into sentrygin, which captures a panic itself before
// honoring Repanic and would therefore bill two events for one failure.
func TestRouter_APanickingRequestYieldsExactlyOneEvent(t *testing.T) {
	transport := bindRecordingHub(t)

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(validCookie())
	rec := httptest.NewRecorder()
	newGatewayEngine(t, panickingValidator{}).ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	events := transport.Events()
	require.Len(t, events, 1, "two events means the recovery re-panicked into sentrygin")
	assert.Equal(t, []string{"{{ default }}", "panic.http"}, events[0].Fingerprint)
	assert.Equal(t, "internal", events[0].Tags["error_kind"])
}

// TestRouter_ProbeEndpointsYieldNoEvents guards the quota floor. Docker probes and
// Prometheus scrapes reach these paths on the order of 86,400 times a day across
// the stack, against an allowance of 5,000 events a month shared org-wide.
func TestRouter_ProbeEndpointsYieldNoEvents(t *testing.T) {
	transport := bindRecordingHub(t)
	engine := newGatewayEngine(t, userValidator())

	for _, path := range []string{"/health", "/metrics", "/readyz"} {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assert.NotEqual(t, http.StatusNotFound, rec.Code, path)
	}

	assert.Empty(t, transport.Events(),
		"a failing readiness probe is a downstream state, not a gateway defect")
}

// TestRouter_ARecordedPanicNamesTheSiteOnce pins the log side of the report:
// LogRecoveredPanic's own record carries the panic value and the stack, and the
// errkit record beside it carries the taxonomy, so a query on either finds the
// panic and neither duplicates the other's attributes.
func TestRouter_ARecordedPanicNamesTheSiteOnce(t *testing.T) {
	logger, logs := serverkittest.NewLogger()
	bindRecordingHub(t)

	target, err := url.Parse("http://127.0.0.1:1")
	require.NoError(t, err)
	engine := router.New(panickingValidator{}, &router.ServiceURLs{
		AuthREST:       target,
		ExpenseREST:    target,
		FinanceREST:    target,
		DatarightsREST: target,
	}, sharedaccess.Prefixes(), logger, false)

	req := httptest.NewRequest(http.MethodGet, "/api/finance/periods", nil)
	req.AddCookie(validCookie())
	engine.ServeHTTP(httptest.NewRecorder(), req)

	records, err := logs.ErrorRecords()
	require.NoError(t, err)
	require.Len(t, records, 1,
		"the errkit record goes to slog.Default(), so only the panic record reaches an injected logger")
	assert.Contains(t, records[0], "panic")
	assert.Contains(t, records[0], "stack")
}
