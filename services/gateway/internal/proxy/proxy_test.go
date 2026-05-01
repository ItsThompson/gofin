package proxy_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/gateway/internal/proxy"
)

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewServiceProxy_ForwardsRequestToDownstream(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer downstream.Close()

	target, _ := url.Parse(downstream.URL)
	handler := proxy.NewServiceProxy(target, newSilentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}

func TestNewServiceProxy_PreservesQueryParams(t *testing.T) {
	var capturedQuery string

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()

	target, _ := url.Parse(downstream.URL)
	handler := proxy.NewServiceProxy(target, newSilentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/expenses?year=2026&month=5", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "year=2026&month=5", capturedQuery)
}

func TestNewServiceProxy_PreservesCookies(t *testing.T) {
	var capturedCookies []*http.Cookie

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCookies = r.Cookies()
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()

	target, _ := url.Parse(downstream.URL)
	handler := proxy.NewServiceProxy(target, newSilentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "test-token"})
	req.AddCookie(&http.Cookie{Name: "gofin_refresh", Value: "refresh-token"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, capturedCookies, 2)
	assert.Equal(t, "gofin_access", capturedCookies[0].Name)
	assert.Equal(t, "gofin_refresh", capturedCookies[1].Name)
}

func TestNewServiceProxy_PreservesCustomHeaders(t *testing.T) {
	var capturedUserID, capturedRole string

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = r.Header.Get("X-User-ID")
		capturedRole = r.Header.Get("X-User-Role")
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()

	target, _ := url.Parse(downstream.URL)
	handler := proxy.NewServiceProxy(target, newSilentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.Header.Set("X-User-Role", "admin")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "user-123", capturedUserID)
	assert.Equal(t, "admin", capturedRole)
}

func TestNewServiceProxy_InjectsXForwardedFor(t *testing.T) {
	var capturedForwardedFor string

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedForwardedFor = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()

	target, _ := url.Parse(downstream.URL)
	handler := proxy.NewServiceProxy(target, newSilentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, capturedForwardedFor, "192.168.1.100")
}

func TestNewServiceProxy_Returns502_WhenDownstreamUnreachable(t *testing.T) {
	// Point to an address where nothing is listening.
	target, _ := url.Parse("http://127.0.0.1:19999")
	handler := proxy.NewServiceProxy(target, newSilentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusBadGateway, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "BAD_GATEWAY")
}

func TestNewServiceProxy_PreservesDownstreamResponseHeaders(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Response", "hello")
		w.Header().Set("Set-Cookie", "downstream_cookie=value; Path=/")
		w.WriteHeader(http.StatusCreated)
	}))
	defer downstream.Close()

	target, _ := url.Parse(downstream.URL)
	handler := proxy.NewServiceProxy(target, newSilentLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "hello", recorder.Header().Get("X-Custom-Response"))
	assert.Contains(t, recorder.Header().Get("Set-Cookie"), "downstream_cookie")
}
