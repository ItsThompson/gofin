package proxy

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// NewServiceProxy creates a reverse proxy that forwards requests to the given
// target URL. It preserves cookies, request body, query params, and injects
// X-Forwarded-For. On downstream failure, it returns 502 with a log entry.
func NewServiceProxy(target *url.URL, logger *slog.Logger) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize the Director to preserve the original request properties
	// while targeting the downstream service.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Inject X-Forwarded-For with the client's IP.
		clientIP := req.Header.Get("X-Forwarded-For")
		if remoteIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			if clientIP == "" {
				clientIP = remoteIP
			} else {
				clientIP = clientIP + ", " + remoteIP
			}
		}
		req.Header.Set("X-Forwarded-For", clientIP)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		logger.Error("downstream service unreachable",
			slog.String("target", target.String()),
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
			slog.String("error", err.Error()),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"code":"BAD_GATEWAY","message":"Downstream service is unavailable"}`))
	}

	// Use a transport with reasonable timeouts for inter-service communication.
	proxy.Transport = &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
	}

	return proxy
}
