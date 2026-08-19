package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ConversionRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fx_conversion_requests_total",
			Help: "Total number of FX conversion requests by currency pair and result",
		},
		[]string{"source_currency", "target_currency", "result"},
	)

	ProviderRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fx_provider_requests_total",
			Help: "Total number of Open Exchange Rates requests by result and status code",
		},
		[]string{"result", "status_code"},
	)

	CacheHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fx_cache_hits_total",
			Help: "Total number of fresh FX provider snapshot cache hits",
		},
	)

	CacheMissesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fx_cache_misses_total",
			Help: "Total number of FX provider snapshot cache misses",
		},
	)

	ConversionLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fx_conversion_latency_seconds",
			Help:    "FX conversion latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source_currency", "target_currency"},
	)

	ProviderLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fx_provider_latency_seconds",
			Help:    "Open Exchange Rates provider request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"result"},
	)
)
