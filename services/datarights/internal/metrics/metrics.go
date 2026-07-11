// Package metrics defines custom Prometheus business metrics for the
// datarights service. These are registered separately from the shared
// services/metrics package (which handles HTTP/gRPC middleware metrics).
package metrics

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// Job lifecycle counters
// ---------------------------------------------------------------------------

var (
	// ExportJobsCreatedTotal counts export jobs submitted via the API.
	ExportJobsCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "export_jobs_created_total",
			Help: "Total number of export jobs created",
		},
	)

	// ExportJobsCompletedTotal counts export jobs reaching a terminal state.
	ExportJobsCompletedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "export_jobs_completed_total",
			Help: "Total number of export jobs reaching terminal state",
		},
		[]string{"status"},
	)

	// ExportRateLimitRejectionsTotal counts requests rejected by 30-day cooldown.
	ExportRateLimitRejectionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "export_rate_limit_rejections_total",
			Help: "Total number of requests rejected due to 30-day cooldown",
		},
	)
)

// ---------------------------------------------------------------------------
// Duration histograms
// ---------------------------------------------------------------------------

// Custom bucket definitions for duration metrics.
var jobDurationBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300}

var (
	// ExportJobDurationSeconds observes end-to-end job duration.
	ExportJobDurationSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "export_job_duration_seconds",
			Help:    "End-to-end export job duration in seconds (pending to terminal)",
			Buckets: jobDurationBuckets,
		},
	)

	// ExportDataCollectionDurationSeconds observes per-provider data collection latency.
	ExportDataCollectionDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "export_data_collection_duration_seconds",
			Help:    "Time to collect data from each provider in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider"},
	)

	// ExportEmailSendDurationSeconds observes email delivery latency.
	ExportEmailSendDurationSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "export_email_send_duration_seconds",
			Help:    "Time to send export email via Resend in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
)

// ---------------------------------------------------------------------------
// Size histogram
// ---------------------------------------------------------------------------

// Custom bucket definitions for file size metrics (bytes).
var zipSizeBuckets = []float64{1024, 10240, 102400, 1048576, 10485760, 26214400}

var (
	// ExportZipSizeBytes observes the size of generated ZIP files.
	ExportZipSizeBytes = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "export_zip_size_bytes",
			Help:    "Size of generated ZIP export files in bytes",
			Buckets: zipSizeBuckets,
		},
	)
)

// ---------------------------------------------------------------------------
// Pool gauges
// ---------------------------------------------------------------------------

// poolAccessors reads live pool telemetry. The export engine's jobrunner.Pool
// owns the semaphore, so the pool gauges are exposed as GaugeFuncs that read
// pool.ActiveJobs()/QueuedJobs() at scrape time rather than being Inc/Dec'd.
type poolAccessors struct {
	active func() int
	queued func() int
}

// poolStats holds the live accessors. It defaults to zero-returning stubs so the
// gauges are registered (and report 0) before SetPoolStats wires the real pool.
var poolStats atomic.Pointer[poolAccessors]

func init() {
	poolStats.Store(&poolAccessors{
		active: func() int { return 0 },
		queued: func() int { return 0 },
	})

	// ExportPoolActiveJobs reports the number of currently running export jobs.
	promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "export_pool_active_jobs",
			Help: "Number of currently running export jobs",
		},
		func() float64 { return float64(poolStats.Load().active()) },
	)

	// ExportPoolQueuedJobs reports the number of export jobs waiting for a slot.
	promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "export_pool_queued_jobs",
			Help: "Number of export jobs waiting for a pool slot",
		},
		func() float64 { return float64(poolStats.Load().queued()) },
	)
}

// SetPoolStats wires the live export-pool accessors for the pool gauges. It is
// called once from main after the export engine is constructed; before that the
// gauges report 0.
func SetPoolStats(active, queued func() int) {
	poolStats.Store(&poolAccessors{active: active, queued: queued})
}
