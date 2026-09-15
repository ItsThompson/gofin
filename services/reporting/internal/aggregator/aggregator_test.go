package aggregator

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/reporting/internal/collector"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

func TestBuildWindowSetUsesOverlappingTrailingWindow(t *testing.T) {
	start := time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)
	windows, err := BuildWindowSet(start)
	if err != nil {
		t.Fatalf("BuildWindowSet: %v", err)
	}
	if got := windows.GetTrailingFourWeeks().GetStart().AsTime(); !got.Equal(start.Add(-28 * 24 * time.Hour)) {
		t.Fatalf("trailing start = %s", got)
	}
	if got := windows.GetTrailingFourWeeks().GetEnd().AsTime(); !got.Equal(start) {
		t.Fatalf("trailing end = %s, want report start", got)
	}
}

func TestCompareKeepsSignedDeltaAndNADenominator(t *testing.T) {
	got := Compare(12, 9)
	if got.Delta != 3 || got.Percent == nil || *got.Percent != 33.3 {
		t.Fatalf("Compare(12, 9) = %+v", got)
	}
	zero := Compare(3, 0)
	if zero.Delta != 3 || zero.Percent != nil {
		t.Fatalf("Compare(3, 0) = %+v", zero)
	}
}

func TestRoundHalfAway(t *testing.T) {
	for _, test := range []struct {
		value float64
		want  float64
	}{
		{value: 1.25, want: 1.3},
		{value: -1.25, want: -1.3},
		{value: 1.24, want: 1.2},
	} {
		if got := RoundHalfAway(test.value, 1); got != test.want {
			t.Errorf("RoundHalfAway(%v) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestWarningsAreStableAndDoNotExposeErrorText(t *testing.T) {
	collection := collector.Collection{Groups: []collector.GroupResult{
		{Name: "finance", Err: &collector.RPCError{Group: "finance", Cause: errors.New("postgres password at https://private.example")}},
		{Name: "auth", Metrics: []collector.MetricResult{{Name: "new_users", UnavailableCode: reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED}}},
	}}
	report := Aggregate(collection, nil)
	if len(report.Warnings) != 2 {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
	if strings.Contains(strings.Join(report.Warnings, " "), "private.example") {
		t.Fatalf("warnings contain raw error: %#v", report.Warnings)
	}
	if report.Groups[0].Name != "auth" || report.Groups[1].Name != "finance" {
		t.Fatalf("groups = %#v", report.Groups)
	}
}
