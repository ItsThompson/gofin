// Package metrics defines custom Prometheus business metrics for the
// datarights service. These are registered separately from the shared
// services/metrics package (which handles HTTP/gRPC middleware metrics).
package metrics

import (
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

var (
	// ExportPoolActiveJobs tracks currently running export goroutines.
	ExportPoolActiveJobs = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "export_pool_active_jobs",
			Help: "Number of currently running export jobs",
		},
	)

	// ExportPoolQueuedJobs tracks jobs waiting for a pool slot.
	ExportPoolQueuedJobs = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "export_pool_queued_jobs",
			Help: "Number of export jobs waiting for a pool slot",
		},
	)
)
