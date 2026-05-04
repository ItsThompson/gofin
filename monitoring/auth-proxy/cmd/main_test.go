package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-jwt-secret"

const testAppURL = "http://localhost:3000"

// generateTestToken creates a signed JWT for testing.
func generateTestToken(t *testing.T, role, username string, expiresAt time.Time) string {
	t.Helper()
	claims := accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Role:     role,
		Username: username,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	require.NoError(t, err)
	return token
}

// newTestProxy creates a handler and a backend server that records proxied requests.
func newTestProxy(t *testing.T) (http.Handler, *httptest.Server) {
	t.Helper()

	// Backend mimics Grafana: echoes back the X-WEBAUTH-USER header it received.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-WEBAUTH-USER")
		w.Header().Set("X-Received-User", user)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("grafana-ok"))
	}))
	t.Cleanup(backend.Close)

	proxy := newReverseProxy(backend.URL)
	handler := authHandler([]byte(testSecret), proxy, testAppURL)
	return handler, backend
}

func TestValidAdminPassthrough(t *testing.T) {
	handler, _ := newTestProxy(t)

	token := generateTestToken(t, "admin", "alice", time.Now().Add(15*time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: token})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice", rec.Header().Get("X-Received-User"))
	assert.Equal(t, "grafana-ok", rec.Body.String())
}

func TestNonAdminRejection(t *testing.T) {
	handler, _ := newTestProxy(t)

	token := generateTestToken(t, "user", "bob", time.Now().Add(15*time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: token})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "Access Denied")
	assert.Contains(t, rec.Body.String(), "restricted to administrators")
}

func TestExpiredTokenRejection(t *testing.T) {
	handler, _ := newTestProxy(t)

	token := generateTestToken(t, "admin", "alice", time.Now().Add(-1*time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: token})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "expired or is invalid")
}

func TestMissingCookie(t *testing.T) {
	handler, _ := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "No access token found")
}

func TestInvalidTokenSignature(t *testing.T) {
	handler, _ := newTestProxy(t)

	// Sign with a different secret.
	claims := accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
		Role:     "admin",
		Username: "alice",
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("wrong-secret"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: token})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGarbageTokenRejection(t *testing.T) {
	handler, _ := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "not-a-jwt"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestProxyForwardsRequestPath(t *testing.T) {
	// Backend that echoes the request path.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.Path)
	}))
	t.Cleanup(backend.Close)

	proxy := newReverseProxy(backend.URL)
	handler := authHandler([]byte(testSecret), proxy, testAppURL)

	token := generateTestToken(t, "admin", "alice", time.Now().Add(15*time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/d/system-overview?orgId=1", nil)
	req.AddCookie(&http.Cookie{Name: "gofin_access", Value: token})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/d/system-overview", rec.Body.String())
}

// Verify the error pages are valid HTML with proper structure.
func TestErrorPageHTML(t *testing.T) {
	handler, _ := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "<html")
	assert.Contains(t, body, testAppURL)
	assert.Contains(t, body, "Back to gofin")
}

