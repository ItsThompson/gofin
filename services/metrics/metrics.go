// Package metrics provides shared Prometheus instrumentation for all gofin
// Go services: HTTP middleware, gRPC interceptors, and custom business metric
// definitions.
package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ---------------------------------------------------------------------------
// Standard HTTP metrics
// ---------------------------------------------------------------------------

var (
	// HTTPRequestsTotal counts HTTP requests by method, path, and status code.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration observes HTTP request latency in seconds.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

// ---------------------------------------------------------------------------
// Standard gRPC metrics
// ---------------------------------------------------------------------------

var (
	// GRPCRequestsTotal counts gRPC requests by method and status.
	GRPCRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)

	// GRPCRequestDuration observes gRPC call latency in seconds.
	GRPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
)

// ---------------------------------------------------------------------------
// Recovered panics
// ---------------------------------------------------------------------------

// RecoveredPanicsTotal counts panics recovered anywhere in a service, by the
// site that recovered them.
//
// serverkit.LogRecoveredPanic increments it, rather than an interceptor doing so,
// because that helper is the sole recovery reporter: an interceptor would see the
// two request-scoped gRPC paths and miss the HTTP middleware and every goroutine
// guard. Its site argument comes from a bounded set fixed at compile time, which
// is what keeps the label safe to use. No service label is needed, because
// Prometheus adds job from the scrape config.
var RecoveredPanicsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "recovered_panics_total",
		Help: "Total number of recovered panics, by recovery site",
	},
	[]string{"site"},
)

// ---------------------------------------------------------------------------
// Custom business metrics
// ---------------------------------------------------------------------------

var (
	// ExpenseEntriesTotal counts expense entries created.
	ExpenseEntriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "expense_entries_total",
			Help: "Total number of expense entries created",
		},
	)

	// CorrectionsTotal counts expense corrections created.
	CorrectionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "corrections_total",
			Help: "Total number of expense corrections created",
		},
	)

	// TokenRefreshTotal counts token refresh attempts by outcome.
	TokenRefreshTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "token_refresh_total",
			Help: "Total number of token refresh attempts",
		},
		[]string{"status"},
	)
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// Register adds the /metrics endpoint to a Gin engine, serving the default
// Prometheus registry in exposition format.
func Register(r *gin.Engine) {
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
