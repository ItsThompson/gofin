package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/reporting/internal/collector"
	"github.com/ItsThompson/gofin/services/reporting/internal/config"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

type appGroup struct {
	name  string
	group collector.GroupResult
}

func (g appGroup) Name() string { return g.name }
func (g appGroup) Collect(context.Context, *reportingpb.ReportWindowSet) collector.GroupResult {
	return g.group
}

type appSender struct {
	calls int
	err   error
}

func (s *appSender) Send(context.Context, string) error {
	s.calls++
	return s.err
}

func completeGroups(metric collector.MetricResult) []collector.GroupClient {
	groups := make([]collector.GroupClient, 0, 4)
	for _, name := range []string{"auth", "expense", "finance", "datarights"} {
		groups = append(groups, appGroup{name: name, group: collector.GroupResult{
			Name:    name,
			Metrics: []collector.MetricResult{metric},
		}})
	}
	return groups
}

func availableMetric() collector.MetricResult {
	return collector.MetricResult{Name: "new_users", Values: &reportingpb.CountValues{ReportWeek: 12, PreviousWeek: 9, TrailingFourWeeksTotal: 38}}
}

func testConfig() config.Config {
	return config.Config{RPCTimeout: time.Second, CollectionTimeout: time.Second, DeliveryTimeout: time.Second, AppTimeout: time.Second}
}

func TestRunRendersAllRequiredMetricsInOrder(t *testing.T) {
	groupMetrics := []struct {
		name    string
		metrics []string
	}{
		{name: "auth", metrics: []string{"new_users", "onboarding_completions"}},
		{name: "expense", metrics: []string{"total_expenses", "manual_expenses", "correction_expenses", "prorata_expenses", "active_expenses"}},
		{name: "finance", metrics: []string{"budget_periods", "tags", "prorata_schedules"}},
		{name: "datarights", metrics: []string{"completed_exports"}},
	}
	groups := make([]collector.GroupClient, 0, len(groupMetrics))
	for _, group := range groupMetrics {
		metrics := make([]collector.MetricResult, 0, len(group.metrics))
		for _, name := range group.metrics {
			metrics = append(metrics, collector.MetricResult{Name: name, Values: &reportingpb.CountValues{ReportWeek: 1, PreviousWeek: 1, TrailingFourWeeksTotal: 4}})
		}
		groups = append(groups, appGroup{name: group.name, group: collector.GroupResult{Name: group.name, Metrics: metrics}})
	}
	var output bytes.Buffer
	result, err := Run(context.Background(), "2026-09-07", true, testConfig(), Dependencies{
		Groups: groups,
		Output: &output,
		Now:    func() time.Time { return time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	last := -1
	for _, name := range []string{
		"New users", "Onboarding completions", "Total expenses", "Manual expenses", "Correction expenses",
		"Pro-rata expenses", "Active expenses", "Budget periods", "Tags", "Pro-rata schedules", "Completed exports",
	} {
		position := strings.Index(output.String(), name+":")
		if position <= last {
			t.Fatalf("metric %q is missing or out of order in report:\n%s", name, output.String())
		}
		last = position
	}
	if result.Partial {
		t.Fatal("complete metric fixture returned a partial result")
	}
}

func TestRunDeliversPartialReportBeforeReturningFailure(t *testing.T) {
	sender := &appSender{}
	var output bytes.Buffer
	result, err := Run(context.Background(), "2026-09-07", false, testConfig(), Dependencies{
		Groups: completeGroups(collector.MetricResult{Name: "new_users", UnavailableCode: reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED}),
		Sender: sender,
		Output: &output,
		Now:    func() time.Time { return time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC) },
	})
	if err == nil {
		t.Fatal("expected partial result error")
	}
	if !result.Partial || sender.calls != 1 || output.Len() == 0 {
		t.Fatalf("result=%+v calls=%d output=%d", result, sender.calls, output.Len())
	}
	var partial *PartialError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %T, want PartialError", err)
	}
}

func TestRunLogsCollectionDurationAndGroupOutcomes(t *testing.T) {
	var output bytes.Buffer
	var logs bytes.Buffer
	result, err := Run(context.Background(), "2026-09-07", true, testConfig(), Dependencies{
		Groups: completeGroups(availableMetric()),
		Output: &output,
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		Now:    func() time.Time { return time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Text == "" {
		t.Fatal("expected report text")
	}
	var collectionEvent map[string]any
	var authEvent map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid structured log %q: %v", line, err)
		}
		switch event["msg"] {
		case "reporting collection completed":
			collectionEvent = event
		case "reporting group collection completed":
			if event["service"] == "auth" {
				authEvent = event
			}
		}
	}
	if collectionEvent["duration"] == nil || collectionEvent["group_count"] != float64(4) {
		t.Fatalf("collection log = %v", collectionEvent)
	}
	if authEvent["outcome"] != "available" || authEvent["metric_count"] != float64(1) {
		t.Fatalf("auth log = %v", authEvent)
	}
}

func TestRunReturnsDeliveryFailureBeforePartialStatus(t *testing.T) {
	sender := &appSender{err: errors.New("transport failed")}
	var output bytes.Buffer
	result, err := Run(context.Background(), "2026-09-07", false, testConfig(), Dependencies{
		Groups: completeGroups(collector.MetricResult{UnavailableCode: reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED}),
		Sender: sender,
		Output: &output,
		Now:    func() time.Time { return time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC) },
	})

	if err == nil || !result.Partial || result.Delivered || sender.calls != 1 || output.Len() == 0 {
		t.Fatalf("result=%+v calls=%d output=%d err=%v", result, sender.calls, output.Len(), err)
	}
	var lifecycle *LifecycleError
	if !errors.As(err, &lifecycle) || lifecycle.Stage != "discord" {
		t.Fatalf("error = %v, want discord lifecycle error", err)
	}
}

func TestRunDryRunDoesNotConstructOrCallSender(t *testing.T) {
	var output bytes.Buffer
	result, err := Run(context.Background(), "2026-09-07", true, testConfig(), Dependencies{
		Groups: completeGroups(availableMetric()),
		Output: &output,
		Now:    func() time.Time { return time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Delivered || !strings.Contains(output.String(), "Growth") {
		t.Fatalf("result=%+v output=%s", result, output.String())
	}
	if !strings.Contains(output.String(), "Generated: 2026-09-14T00:00:00 UTC") {
		t.Fatalf("generated timestamp did not use injected clock: %s", output.String())
	}
}

func TestRunRequiresOutputBeforeDelivery(t *testing.T) {
	sender := &appSender{}
	result, err := Run(context.Background(), "2026-09-07", false, testConfig(), Dependencies{
		Groups: completeGroups(availableMetric()),
		Sender: sender,
		Now:    func() time.Time { return time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC) },
	})
	if err == nil || result.Text == "" || sender.calls != 0 {
		t.Fatalf("result=%+v sender calls=%d err=%v", result, sender.calls, err)
	}
	var lifecycle *LifecycleError
	if !errors.As(err, &lifecycle) || lifecycle.Stage != "output" {
		t.Fatalf("error = %v, want output lifecycle error", err)
	}
}

func TestRunPrintsBeforeSizeValidation(t *testing.T) {
	longMetrics := make([]collector.MetricResult, 0, 100)
	for i := 0; i < 100; i++ {
		longMetrics = append(longMetrics, collector.MetricResult{Name: strings.Repeat("metric", 5), Values: &reportingpb.CountValues{ReportWeek: int64(i), PreviousWeek: 1, TrailingFourWeeksTotal: 4}})
	}
	groups := make([]collector.GroupClient, 0, 4)
	for _, name := range []string{"auth", "expense", "finance", "datarights"} {
		groups = append(groups, appGroup{name: name, group: collector.GroupResult{Name: name, Metrics: longMetrics}})
	}
	var output bytes.Buffer
	result, err := Run(context.Background(), "2026-09-07", true, testConfig(), Dependencies{Groups: groups, Output: &output})
	if err == nil {
		t.Fatal("expected size error")
	}
	if output.Len() == 0 || result.Text == "" {
		t.Fatal("report was not printed before size validation")
	}
	var lifecycle *LifecycleError
	if !errors.As(err, &lifecycle) || lifecycle.Stage != "size" {
		t.Fatalf("error = %v", err)
	}
}
