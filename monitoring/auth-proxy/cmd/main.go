package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// accessTokenClaims mirrors the auth service's access token structure.
// Defined locally: the proxy is intentionally independent of the auth service module.
type accessTokenClaims struct {
	jwt.RegisteredClaims
	Role     string `json:"role"`
	Username string `json:"username"`
}

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	grafanaURL := os.Getenv("GRAFANA_URL")
	if grafanaURL == "" {
		log.Fatal("GRAFANA_URL environment variable is required")
	}

	proxy := newReverseProxy(grafanaURL)

	http.HandleFunc("/", authHandler([]byte(jwtSecret), proxy))

	addr := ":3002"
	log.Printf("grafana-auth-proxy listening on %s, forwarding to %s", addr, grafanaURL)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// authHandler returns an HTTP handler that validates the JWT cookie and proxies to Grafana.
func authHandler(secret []byte, proxy *httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("gofin_access")
		if err != nil {
			renderError(w, http.StatusUnauthorized, "Authentication Required",
				"No access token found. Please log in to the gofin application first.")
			return
		}

		claims, err := validateToken(cookie.Value, secret)
		if err != nil {
			renderError(w, http.StatusUnauthorized, "Authentication Failed",
				"Your session has expired or is invalid. Please log in again.")
			return
		}

		if claims.Role != "admin" {
			renderError(w, http.StatusForbidden, "Access Denied",
				"Grafana dashboards are restricted to administrators. Contact your admin for access.")
			return
		}

		r.Header.Set("X-WEBAUTH-USER", claims.Username)
		proxy.ServeHTTP(w, r)
	}
}

// validateToken parses and validates a JWT access token.
func validateToken(tokenString string, secret []byte) (*accessTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &accessTokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*accessTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// renderError writes a user-friendly HTML error page.
func renderError(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s - gofin</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #e2e8f0; display: flex; align-items: center; justify-content: center; min-height: 100vh; }
    .card { background: #1e293b; border-radius: 12px; padding: 2.5rem; max-width: 420px; text-align: center; box-shadow: 0 4px 24px rgba(0,0,0,0.3); }
    .icon { font-size: 3rem; margin-bottom: 1rem; }
    h1 { font-size: 1.5rem; margin-bottom: 0.75rem; color: #f8fafc; }
    p { color: #94a3b8; line-height: 1.6; margin-bottom: 1.5rem; }
    a { display: inline-block; background: #3b82f6; color: #fff; text-decoration: none; padding: 0.625rem 1.5rem; border-radius: 6px; font-weight: 500; transition: background 0.2s; }
    a:hover { background: #2563eb; }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon">%s</div>
    <h1>%s</h1>
    <p>%s</p>
    <a href="/">← Back to gofin</a>
  </div>
</body>
</html>`, title, statusIcon(status), title, message)
}

// newReverseProxy creates a reverse proxy for the given target URL.
func newReverseProxy(targetURL string) *httputil.ReverseProxy {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("invalid proxy target URL: %v", err)
	}
	return httputil.NewSingleHostReverseProxy(target)
}

// statusIcon returns an emoji for the given HTTP status code.
func statusIcon(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "🔒"
	case http.StatusForbidden:
		return "🚫"
	default:
		return "⚠️"
	}
}
