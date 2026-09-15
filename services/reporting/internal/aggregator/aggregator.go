package aggregator

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ItsThompson/gofin/services/reporting/internal/collector"
	sharedreporting "github.com/ItsThompson/gofin/services/shared/reporting"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	WarningLimit = 256
	WeekCount    = 4
)

type Comparison struct {
	Delta   int64
	Percent *float64
}

type Metric struct {
	Group       string
	Name        string
	Available   bool
	Current     int64
	Previous    int64
	FourWeekSum int64
	Average     float64
	Comparison  Comparison
	ErrorCode   reportingpb.MetricErrorCode
}

type Group struct {
	Name    string
	Metrics []Metric
	Err     error
}

type Report struct {
	WindowSet   *reportingpb.ReportWindowSet
	GeneratedAt time.Time
	Groups      []Group
	Warnings    []string
}

func BuildWindowSet(reportStart time.Time) (*reportingpb.ReportWindowSet, error) {
	start := reportStart.UTC()
	if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 || start.Nanosecond() != 0 {
		return nil, fmt.Errorf("report start must be midnight UTC")
	}
	if start.Weekday() != time.Monday {
		return nil, fmt.Errorf("report start must be Monday")
	}
	week := 7 * 24 * time.Hour
	reportEnd := start.Add(week)
	set := &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(start),
		ReportWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(start),
			End:   timestamppb.New(reportEnd),
		},
		PreviousWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(start.Add(-week)),
			End:   timestamppb.New(start),
		},
		TrailingFourWeeks: &reportingpb.UtcWindow{
			Start: timestamppb.New(start.Add(-4 * week)),
			End:   timestamppb.New(start),
		},
	}
	if err := sharedreporting.ValidateWindowSet(set); err != nil {
		return nil, err
	}
	return set, nil
}

func Aggregate(collection collector.Collection, windows *reportingpb.ReportWindowSet) Report {
	report := Report{WindowSet: windows, Groups: make([]Group, 0, len(collection.Groups))}
	for _, source := range collection.Groups {
		group := Group{Name: source.Name, Err: source.Err, Metrics: make([]Metric, 0, len(source.Metrics))}
		if source.Err != nil {
			report.Warnings = append(report.Warnings, warningForGroup(source))
		}
		for _, sourceMetric := range source.Metrics {
			metric := Metric{
				Group:     source.Name,
				Name:      sourceMetric.Name,
				ErrorCode: sourceMetric.UnavailableCode,
			}
			if sourceMetric.Values == nil {
				report.Warnings = append(report.Warnings, warningForMetric(source.Name, sourceMetric))
				group.Metrics = append(group.Metrics, metric)
				continue
			}
			metric.Available = true
			metric.Current = sourceMetric.Values.GetReportWeek()
			metric.Previous = sourceMetric.Values.GetPreviousWeek()
			metric.FourWeekSum = sourceMetric.Values.GetTrailingFourWeeksTotal()
			metric.Average = RoundHalfAway(float64(metric.FourWeekSum)/float64(WeekCount), 1)
			metric.Comparison = Compare(metric.Current, metric.Previous)
			group.Metrics = append(group.Metrics, metric)
		}
		report.Groups = append(report.Groups, group)
	}
	sort.SliceStable(report.Groups, func(i, j int) bool {
		return groupRank(report.Groups[i].Name) < groupRank(report.Groups[j].Name)
	})
	report.Warnings = StableWarnings(report.Warnings)
	return report
}

func Compare(current, previous int64) Comparison {
	comparison := Comparison{Delta: current - previous}
	if previous != 0 {
		percent := float64(comparison.Delta) / float64(previous) * 100
		percent = RoundHalfAway(percent, 1)
		comparison.Percent = &percent
	}
	return comparison
}

func RoundHalfAway(value float64, places int) float64 {
	if places < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	factor := math.Pow10(places)
	if value < 0 {
		return math.Ceil(value*factor-0.5) / factor
	}
	return math.Floor(value*factor+0.5) / factor
}

func StableWarnings(warnings []string) []string {
	seen := make(map[string]struct{}, len(warnings))
	stable := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		warning = SanitizeWarning(warning)
		if warning == "" {
			continue
		}
		if _, exists := seen[warning]; exists {
			continue
		}
		seen[warning] = struct{}{}
		stable = append(stable, warning)
	}
	sort.Strings(stable)
	return stable
}

func SanitizeWarning(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	spacePending := false
	for _, char := range value {
		if unicode.IsSpace(char) || unicode.IsControl(char) {
			spacePending = builder.Len() > 0
			continue
		}
		if spacePending {
			builder.WriteByte(' ')
			spacePending = false
		}
		builder.WriteRune(char)
	}
	value = strings.TrimSpace(builder.String())
	if len([]rune(value)) > WarningLimit {
		value = string([]rune(value)[:WarningLimit])
	}
	return value
}

func warningForGroup(group collector.GroupResult) string {
	if group.Err == nil {
		return ""
	}
	code := "GROUP_FAILED"
	if _, ok := group.Err.(*collector.RPCError); ok {
		code = "GROUP_RPC_FAILED"
	} else if _, ok := group.Err.(*collector.WindowError); ok {
		code = "GROUP_WINDOW_INVALID"
	}
	return fmt.Sprintf("%s unavailable: %s", group.Name, code)
}

func warningForMetric(group string, metric collector.MetricResult) string {
	return fmt.Sprintf("%s.%s unavailable: %s", group, metric.Name, metricErrorCode(metric.UnavailableCode))
}

func metricErrorCode(code reportingpb.MetricErrorCode) string {
	switch code {
	case reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED:
		return "QUERY_FAILED"
	case reportingpb.MetricErrorCode_METRIC_ERROR_CODE_INVALID_WINDOW:
		return "INVALID_WINDOW"
	case reportingpb.MetricErrorCode_METRIC_ERROR_CODE_INTERNAL:
		return "INTERNAL"
	default:
		return "UNAVAILABLE"
	}
}

func groupRank(name string) int {
	switch name {
	case "auth":
		return 0
	case "expense":
		return 1
	case "finance":
		return 2
	case "datarights":
		return 3
	default:
		return 4
	}
}
