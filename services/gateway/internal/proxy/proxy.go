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
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host

			// Inject X-Forwarded-For with the client's IP.
			clientIP := r.In.Header.Get("X-Forwarded-For")
			if remoteIP, _, err := net.SplitHostPort(r.In.RemoteAddr); err == nil {
				if clientIP == "" {
					clientIP = remoteIP
				} else {
					clientIP = clientIP + ", " + remoteIP
				}
			}
			r.Out.Header.Set("X-Forwarded-For", clientIP)
		},
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
		_, _ = w.Write([]byte(`{"code":"BAD_GATEWAY","message":"Downstream service is unavailable"}`))
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
