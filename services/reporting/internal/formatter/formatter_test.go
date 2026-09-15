package formatter

import (
	"strings"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/reporting/internal/aggregator"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFormatUsesStableSectionsAndCompactRows(t *testing.T) {
	start := time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)
	report := aggregator.Report{
		WindowSet: &reportingpb.ReportWindowSet{ReportWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(start), End: timestamppb.New(start.Add(7 * 24 * time.Hour)),
		}},
		GeneratedAt: start.Add(8 * time.Hour),
		Groups: []aggregator.Group{
			{Name: "datarights", Metrics: []aggregator.Metric{{Name: "completed_exports", Available: true, Current: 2, Average: 1.5, Comparison: aggregator.Comparison{Delta: -1, Percent: func() *float64 { value := -33.3; return &value }()}}}},
			{Name: "auth", Metrics: []aggregator.Metric{{Name: "new_users", Available: true, Current: 12, Average: 9.5, Comparison: aggregator.Comparison{Delta: 3, Percent: func() *float64 { value := 33.3; return &value }()}}}},
			{Name: "expense", Metrics: []aggregator.Metric{{Name: "total_expenses", Available: false}}},
		},
	}
	text := Format(report)
	if strings.Index(text, "Growth") > strings.Index(text, "Financial activity") || strings.Index(text, "Financial activity") > strings.Index(text, "Exports") {
		t.Fatalf("sections are not ordered: %s", text)
	}
	for _, want := range []string{
		"Report range: 2026-09-07 to 2026-09-14 UTC",
		"Generated: 2026-09-07T08:00:00 UTC",
		"New users: 12 | WoW +3 (+33.3%) | 4wk avg 9.5",
		"Total expenses: N/A | WoW N/A | 4wk avg N/A",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report does not contain %q:\n%s", want, text)
		}
	}
}
