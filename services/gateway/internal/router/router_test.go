package router_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedaccess "github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/access"
	"github.com/ItsThompson/gofin/services/gateway/internal/router"
)

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// mockValidator implements access.TokenValidator for router tests.
type mockValidator struct {
	result *access.TokenValidationResult
	err    error
}

func (m *mockValidator) ValidateToken(_ context.Context, _ string) (*access.TokenValidationResult, error) {
	return m.result, m.err
}

func validCookie() *http.Cookie {
	return &http.Cookie{Name: "gofin_access", Value: "valid-token"}
}

func adminValidator() *mockValidator {
	return &mockValidator{
		result: &access.TokenValidationResult{
			UserID: "admin-1",
			Role:   "admin",
		},
	}
}

func userValidator() *mockValidator {
	return &mockValidator{
		result: &access.TokenValidationResult{
			UserID: "user-1",
			Role:   "user",
		},
	}
}

// setupGateway creates a full gateway test server backed by downstream httptest servers.
// It returns a doRequest helper and cleans up all servers on test completion.
func setupGateway(t *testing.T, validator access.TokenValidator) func(method, path string, cookie *http.Cookie) (*http.Response, string) {
	t.Helper()

	// Each downstream echoes its service name in a header so tests can verify routing.
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Downstream", "auth")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(authServer.Close)

	expenseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Downstream", "expense")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(expenseServer.Close)

	financeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Downstream", "finance")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(financeServer.Close)

	datarightsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Downstream", "datarights")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(datarightsServer.Close)

	authURL, _ := url.Parse(authServer.URL)
	expenseURL, _ := url.Parse(expenseServer.URL)
	financeURL, _ := url.Parse(financeServer.URL)
	datarightsURL, _ := url.Parse(datarightsServer.URL)

	engine := router.New(validator, &router.ServiceURLs{
		AuthREST:       authURL,
		ExpenseREST:    expenseURL,
		FinanceREST:    financeURL,
		DatarightsREST: datarightsURL,
	}, sharedaccess.Prefixes(), newSilentLogger(), false)

	// Use a real HTTP test server so the response writer supports CloseNotifier
	// (required by httputil.ReverseProxy).
	gatewayServer := httptest.NewServer(engine)
	t.Cleanup(gatewayServer.Close)

	client := gatewayServer.Client()
	// Don't follow redirects: we want to inspect the raw response.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return func(method, path string, cookie *http.Cookie) (*http.Response, string) {
		req, _ := http.NewRequest(method, gatewayServer.URL+path, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %s %s failed: %v", method, path, err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		return resp, string(body)
	}
}

func TestRouter_AuthRoutes_RouteToAuthService(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/register"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/refresh"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodGet, "/api/auth/me"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp, _ := doRequest(tt.method, tt.path, validCookie())
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "auth", resp.Header.Get("X-Downstream"))
		})
	}
}

func TestRouter_ExpenseRoutes_RouteToExpenseService(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/expenses"},
		{http.MethodGet, "/api/expenses?year=2026&month=5"},
		{http.MethodGet, "/api/expenses/suggestions?page=1&pageSize=50"},
		{http.MethodGet, "/api/expenses/abc-123"},
		{http.MethodPost, "/api/expenses/abc-123/correct"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp, _ := doRequest(tt.method, tt.path, validCookie())
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "expense", resp.Header.Get("X-Downstream"))
		})
	}
}

func TestRouter_FinanceRoutes_RouteToFinanceService(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/finance/periods/current?year=2026&month=5"},
		{http.MethodPost, "/api/finance/periods"},
		{http.MethodGet, "/api/finance/tags"},
		{http.MethodPost, "/api/finance/prorata"},
		{http.MethodGet, "/api/finance/summary?year=2026&month=5"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp, _ := doRequest(tt.method, tt.path, validCookie())
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "finance", resp.Header.Get("X-Downstream"))
		})
	}
}

func TestRouter_DatarightsRoutes_RouteToDatarightsService(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/datarights/exports"},
		{http.MethodGet, "/api/datarights/exports"},
		{http.MethodGet, "/api/datarights/exports/job-123"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp, _ := doRequest(tt.method, tt.path, validCookie())
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "datarights", resp.Header.Get("X-Downstream"))
		})
	}
}

func TestRouter_AdminRoutes_AdminRolePasses(t *testing.T) {
	doRequest := setupGateway(t, adminValidator())

	resp, _ := doRequest(http.MethodGet, "/api/admin/users", validCookie())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "auth", resp.Header.Get("X-Downstream"))
}

func TestRouter_AdminRoutes_RejectNonAdmin(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	resp, _ := doRequest(http.MethodGet, "/api/admin/users", validCookie())
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestRouter_PersonalRoutes_RejectDirectAdmin is the observable cutover: a
// direct admin (role=="admin", not an assumed session) is now forbidden from
// Personal APIs, where the old admin-as-superset model let them through. The
// request is denied at the gateway and never reaches the downstream service.
// Every path here is a concrete registered route (the resolver classifies exact
// gin patterns, so a non-real trailing-slash path like "/api/expenses/" would
// 404 downstream and falls to the Authenticated default instead).
func TestRouter_PersonalRoutes_RejectDirectAdmin(t *testing.T) {
	doRequest := setupGateway(t, adminValidator())

	personalRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/finance/periods/current"},
		{http.MethodGet, "/api/expenses"},
		{http.MethodPost, "/api/datarights/exports"},
		{http.MethodPost, "/api/auth/onboarding-complete"},
	}

	for _, tt := range personalRoutes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp, body := doRequest(tt.method, tt.path, validCookie())
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			assert.Contains(t, body, "FORBIDDEN")
			assert.Empty(t, resp.Header.Get("X-Downstream"),
				"denied request must not reach a downstream service")
		})
	}
}

func TestRouter_AssumeEndpoint_RequiresAdmin(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	resp, body := doRequest(http.MethodPost, "/api/auth/assume", validCookie())
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Contains(t, body, "FORBIDDEN")
}

func TestRouter_AssumeEndpoint_AdminPasses(t *testing.T) {
	doRequest := setupGateway(t, adminValidator())

	resp, _ := doRequest(http.MethodPost, "/api/auth/assume", validCookie())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "auth", resp.Header.Get("X-Downstream"))
}

func TestRouter_UnauthenticatedRoutes_NoCookieNeeded(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/register"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/refresh"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp, _ := doRequest(tt.method, tt.path, nil)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "auth", resp.Header.Get("X-Downstream"))
		})
	}
}

// TestRouter_AuthenticatedRoute_NoCookie_Returns401 targets a real Authenticated
// route: with no cookie the gateway must 401 (identity required) before any
// proxying. (A non-real path like "/api/expenses/" is now Deny -> 403, so it
// would no longer exercise the missing-cookie 401 path.)
func TestRouter_AuthenticatedRoute_NoCookie_Returns401(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	resp, _ := doRequest(http.MethodGet, "/api/auth/me", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRouter_HealthEndpoint(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	resp, body := doRequest(http.MethodGet, "/health", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, `"status":"ok"`)
}

// TestRouter_ReadyzEndpoint_AllHealthy_Returns200 pins the happy path: with
// every downstream answering /health with 200, /readyz aggregates to 200
// {"status":"ok"} and requires no cookie (Public via GatewayResolve).
func TestRouter_ReadyzEndpoint_AllHealthy_Returns200(t *testing.T) {
	doRequest := setupGateway(t, userValidator())

	resp, body := doRequest(http.MethodGet, "/readyz", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, `"status":"ok"`)
}

// TestRouter_ReadyzEndpoint_DownstreamUnhealthy_Returns503NamingService proves
// /readyz aggregates downstream health and names the failing service in a 503.
// Built directly (not via setupGateway) so one downstream can fail its /health
// while the others stay healthy.
func TestRouter_ReadyzEndpoint_DownstreamUnhealthy_Returns503NamingService(t *testing.T) {
	newHealthy := func() *url.URL {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)
		u, _ := url.Parse(server.URL)
		return u
	}

	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(unhealthy.Close)
	expenseURL, _ := url.Parse(unhealthy.URL)

	engine := router.New(userValidator(), &router.ServiceURLs{
		AuthREST:       newHealthy(),
		ExpenseREST:    expenseURL,
		FinanceREST:    newHealthy(),
		DatarightsREST: newHealthy(),
	}, sharedaccess.Prefixes(), newSilentLogger(), false)

	gatewayServer := httptest.NewServer(engine)
	t.Cleanup(gatewayServer.Close)

	resp, err := gatewayServer.Client().Get(gatewayServer.URL + "/readyz")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Contains(t, string(body), `"status":"unhealthy"`)
	assert.Contains(t, string(body), "expense", "the 503 body must name the unhealthy service")
}

// TestRouter_New_PanicsOnPrefixWithNoProxy pins the data-driven wiring's
// fail-fast contract: because router.New derives its proxy groups from the
// injected prefix inventory, a prefix naming a service with no proxy handler
// is a wiring bug that must panic at construction rather than silently drop the
// prefix. The bad prefix is injected directly (no shared-global mutation).
func TestRouter_New_PanicsOnPrefixWithNoProxy(t *testing.T) {
	badPrefixes := append(sharedaccess.Prefixes(),
		sharedaccess.ProxyPrefix{Prefix: "/api/ghost", Service: "ghost"})

	defer func() {
		r := recover()
		require.NotNil(t, r, "New must panic when a ProxyPrefix names a service with no proxy handler")
		assert.Contains(t, fmt.Sprintf("%v", r), "ghost",
			"panic message must name the offending service")
	}()

	u, _ := url.Parse("http://127.0.0.1:1")
	router.New(userValidator(), &router.ServiceURLs{
		AuthREST:       u,
		ExpenseREST:    u,
		FinanceREST:    u,
		DatarightsREST: u,
	}, badPrefixes, newSilentLogger(), false)
}
