package metrics

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllMetricsRegistered(t *testing.T) {
	// Initialize Vec metrics so they appear in Gather output.
	// Vec types are only visible after at least one label set is used.
	ExportJobsCompletedTotal.WithLabelValues("completed")
	ExportDataCollectionDurationSeconds.WithLabelValues("_init")

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	expectedMetrics := []string{
		"export_jobs_created_total",
		"export_jobs_completed_total",
		"export_job_duration_seconds",
		"export_data_collection_duration_seconds",
		"export_email_send_duration_seconds",
		"export_zip_size_bytes",
		"export_pool_active_jobs",
		"export_pool_queued_jobs",
		"export_rate_limit_rejections_total",
		"export_currency_formatting_fallback_total",
	}

	foundMetrics := make(map[string]bool)
	for _, family := range families {
		foundMetrics[family.GetName()] = true
	}

	for _, name := range expectedMetrics {
		assert.True(t, foundMetrics[name], "metric %q should be registered", name)
	}
}

func TestExportJobsCreatedTotal_Increments(t *testing.T) {
	ExportJobsCreatedTotal.Inc()
	ExportJobsCreatedTotal.Inc()

	value := testutil.ToFloat64(ExportJobsCreatedTotal)
	assert.GreaterOrEqual(t, value, float64(2))
}

func TestExportJobsCompletedTotal_TracksStatus(t *testing.T) {
	ExportJobsCompletedTotal.WithLabelValues("completed").Inc()
	ExportJobsCompletedTotal.WithLabelValues("failed").Inc()
	ExportJobsCompletedTotal.WithLabelValues("failed").Inc()

	completedVal := testutil.ToFloat64(ExportJobsCompletedTotal.WithLabelValues("completed"))
	failedVal := testutil.ToFloat64(ExportJobsCompletedTotal.WithLabelValues("failed"))

	assert.GreaterOrEqual(t, completedVal, float64(1))
	assert.GreaterOrEqual(t, failedVal, float64(2))
}

func TestExportRateLimitRejectionsTotal_Increments(t *testing.T) {
	ExportRateLimitRejectionsTotal.Inc()

	value := testutil.ToFloat64(ExportRateLimitRejectionsTotal)
	assert.GreaterOrEqual(t, value, float64(1))
}

func TestExportCurrencyFormattingFallbackTotal_Increments(t *testing.T) {
	ExportCurrencyFormattingFallbackTotal.Inc()

	value := testutil.ToFloat64(ExportCurrencyFormattingFallbackTotal)
	assert.GreaterOrEqual(t, value, float64(1))
}

func TestExportPoolGauges_ReflectLivePoolStats(t *testing.T) {
	var active, queued atomic.Int64
	SetPoolStats(
		func() int { return int(active.Load()) },
		func() int { return int(queued.Load()) },
	)
	// Restore the zero-returning stubs so other tests are unaffected.
	t.Cleanup(func() {
		SetPoolStats(func() int { return 0 }, func() int { return 0 })
	})

	active.Store(3)
	queued.Store(2)

	assert.Equal(t, float64(3), gaugeValue(t, "export_pool_active_jobs"))
	assert.Equal(t, float64(2), gaugeValue(t, "export_pool_queued_jobs"))

	active.Store(0)
	queued.Store(0)

	assert.Equal(t, float64(0), gaugeValue(t, "export_pool_active_jobs"))
	assert.Equal(t, float64(0), gaugeValue(t, "export_pool_queued_jobs"))
}

// gaugeValue gathers the default registry and returns the first sample of the
// named gauge family.
func gaugeValue(t *testing.T, name string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		require.NotEmpty(t, family.GetMetric())
		return family.GetMetric()[0].GetGauge().GetValue()
	}
	t.Fatalf("metric %q not found", name)
	return 0
}

func TestExportDataCollectionDurationSeconds_RecordsProviderLabel(t *testing.T) {
	ExportDataCollectionDurationSeconds.WithLabelValues("profile").Observe(0.5)
	ExportDataCollectionDurationSeconds.WithLabelValues("expenses").Observe(1.2)

	// Verify that observations for different providers are recorded separately
	// by checking the metric count per label value.
	profileCount := testutil.CollectAndCount(ExportDataCollectionDurationSeconds, "export_data_collection_duration_seconds")
	assert.GreaterOrEqual(t, profileCount, 2, "expected metric samples for both providers")

	// Verify the histogram is populated by gathering and inspecting the family
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var found bool
	for _, family := range families {
		if family.GetName() != "export_data_collection_duration_seconds" {
			continue
		}
		found = true
		// Should have metrics for both "profile" and "expenses" labels
		providers := make(map[string]bool)
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "provider" {
					providers[label.GetValue()] = true
				}
			}
		}
		assert.True(t, providers["profile"], "expected provider=profile label")
		assert.True(t, providers["expenses"], "expected provider=expenses label")
	}
	assert.True(t, found, "expected export_data_collection_duration_seconds metric family")
}

func TestExportJobDurationSeconds_ObservesValues(t *testing.T) {
	ExportJobDurationSeconds.Observe(5.5)
	ExportJobDurationSeconds.Observe(10.2)

	// Verify observations land in correct buckets by gathering the histogram
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != "export_job_duration_seconds" {
			continue
		}
		for _, metric := range family.GetMetric() {
			h := metric.GetHistogram()
			require.NotNil(t, h)
			// We observed 2 values, sample count must reflect that
			assert.GreaterOrEqual(t, h.GetSampleCount(), uint64(2))
			// Sum should be at least 5.5 + 10.2 = 15.7
			assert.GreaterOrEqual(t, h.GetSampleSum(), 15.7)
			// Custom buckets should include 10 (from our bucket definition)
			var hasBucket10 bool
			for _, b := range h.GetBucket() {
				if b.GetUpperBound() == 10 {
					hasBucket10 = true
					// 5.5 falls in the <=10 bucket
					assert.GreaterOrEqual(t, b.GetCumulativeCount(), uint64(1))
				}
			}
			assert.True(t, hasBucket10, "expected custom bucket boundary at 10s")
		}
	}
}

func TestExportZipSizeBytes_CustomBuckets(t *testing.T) {
	ExportZipSizeBytes.Observe(512)     // below first bucket (1024)
	ExportZipSizeBytes.Observe(5000)    // between 1024 and 10240
	ExportZipSizeBytes.Observe(1048576) // exactly at 1MB bucket

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != "export_zip_size_bytes" {
			continue
		}
		for _, metric := range family.GetMetric() {
			h := metric.GetHistogram()
			require.NotNil(t, h)
			assert.GreaterOrEqual(t, h.GetSampleCount(), uint64(3))

			// Verify custom bucket boundaries exist
			bucketBounds := make([]float64, 0, len(h.GetBucket()))
			for _, b := range h.GetBucket() {
				bucketBounds = append(bucketBounds, b.GetUpperBound())
			}
			// Should contain our custom boundaries
			assert.Contains(t, bucketBounds, float64(1024))
			assert.Contains(t, bucketBounds, float64(10240))
			assert.Contains(t, bucketBounds, float64(1048576))
			assert.Contains(t, bucketBounds, float64(26214400))

			// Verify 512 landed in the <=1024 bucket
			for _, b := range h.GetBucket() {
				if b.GetUpperBound() == 1024 {
					// Only 512 is <= 1024 (5000 > 1024)
					assert.GreaterOrEqual(t, b.GetCumulativeCount(), uint64(1))
				}
			}
		}
	}
}

func TestExportEmailSendDurationSeconds_ObservesValues(t *testing.T) {
	ExportEmailSendDurationSeconds.Observe(0.8)
	ExportEmailSendDurationSeconds.Observe(2.1)

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != "export_email_send_duration_seconds" {
			continue
		}
		for _, metric := range family.GetMetric() {
			h := metric.GetHistogram()
			require.NotNil(t, h)
			assert.GreaterOrEqual(t, h.GetSampleCount(), uint64(2))
			assert.GreaterOrEqual(t, h.GetSampleSum(), 2.9) // 0.8 + 2.1
		}
	}
}

func TestNoSensitiveDataInLabels(t *testing.T) {
	// Verify that no metric uses labels that could contain PII.
	// The only label used is "status" and "provider" which are safe.
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	sensitiveLabels := []string{"email", "user_email", "user_name", "name"}

	for _, family := range families {
		if !strings.HasPrefix(family.GetName(), "export_") {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				for _, sensitive := range sensitiveLabels {
					assert.NotEqual(t, sensitive, label.GetName(),
						"metric %q should not have sensitive label %q",
						family.GetName(), sensitive)
				}
			}
		}
	}
}
