package proxy

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
)

// downstreamReportWindow bounds how often one unreachable target is reported.
//
// The error handler runs once per proxied request, not once per outage, so a
// downstream that is down while a single browser tab polls every 2.5 seconds
// produces on the order of 1,440 failures an hour. The monthly event allowance is
// 5,000 and it is shared across the whole organization, so the unbounded form of
// this site spends it in an afternoon. One event an hour per target keeps a
// day-long outage near two dozen events, which is enough to see the shape of the
// incident.
//
// The events are a diagnostic detail rather than the alarm: Prometheus already
// alerts on up == 0 and pages for it.
const downstreamReportWindow = time.Hour

// NewServiceProxy creates a reverse proxy that forwards requests to the given
// target URL. It preserves cookies, request body, query params, and injects
// X-Forwarded-For. On downstream failure, it returns 502 with a log entry.
func NewServiceProxy(target *url.URL, logger *slog.Logger) http.Handler {
	// One limiter per proxy, and the router builds one proxy per downstream, so the
	// bound is per target by construction rather than through a keyed map whose
	// entries nothing would ever remove.
	reports := errkit.NewLimiter(downstreamReportWindow)

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
		// The record stays per request: it is what shows an operator how much traffic
		// the outage is affecting, and log volume is not the constrained resource.
		logger.Error("downstream service unreachable",
			slog.String("target", target.String()),
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
			slog.String("error", err.Error()),
		)

		if reports.Allow() {
			// One issue for the whole class: which target is unreachable is a detail in
			// the context block, and the stack varies with the request while the meaning
			// does not. The outbound request's context descends from the inbound one, so
			// the hub sentrygin installed is still reachable here.
			_ = errkit.Report(req.Context(), err, errkit.Meta{
				Kind:       errkit.KindUpstream,
				Op:         "gateway.proxy",
				Domain:     "platform",
				Msg:        "downstream service unreachable",
				GroupKey:   "gateway.downstream_unreachable",
				GroupExact: true,
				Data: map[string]any{
					"target": target.String(),
					"method": req.Method,
					"path":   req.URL.Path,
				},
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		// Encode via the shared apierr wire struct so every gateway error body
		// (including this raw-ResponseWriter path, which has no gin.Context for
		// apierr.Respond) shares one shape. BAD_GATEWAY is gateway-specific.
		body, _ := json.Marshal(apierr.APIError{
			Code:    "BAD_GATEWAY",
			Message: "Downstream service is unavailable",
		})
		_, _ = w.Write(body)
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
