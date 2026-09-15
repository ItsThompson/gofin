package formatter

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ItsThompson/gofin/services/reporting/internal/aggregator"
)

const MaxReportRunes = 2000

func Format(report aggregator.Report) string {
	var output strings.Builder
	start, end := reportRange(report)
	output.WriteString("GoFin weekly report\n")
	if !start.IsZero() && !end.IsZero() {
		output.WriteString("Report range: ")
		output.WriteString(start.Format("2006-01-02"))
		output.WriteString(" to ")
		output.WriteString(end.Format("2006-01-02"))
		output.WriteString(" UTC\n")
	}
	generated := report.GeneratedAt.UTC()
	output.WriteString("Generated: ")
	output.WriteString(generated.Format("2006-01-02T15:04:05.999999999 UTC"))
	output.WriteByte('\n')

	sections := []struct {
		name   string
		groups []string
	}{
		{name: "Growth", groups: []string{"auth"}},
		{name: "Financial activity", groups: []string{"expense", "finance"}},
		{name: "Exports", groups: []string{"datarights"}},
	}
	for _, section := range sections {
		output.WriteByte('\n')
		output.WriteString(section.name)
		output.WriteByte('\n')
		for _, groupName := range section.groups {
			for _, group := range report.Groups {
				if group.Name != groupName {
					continue
				}
				for _, metric := range group.Metrics {
					writeMetric(&output, metric)
				}
			}
		}
	}

	warnings := aggregator.StableWarnings(report.Warnings)
	if len(warnings) > 0 {
		output.WriteString("\nData warnings\n")
		for _, warning := range warnings {
			output.WriteString("- ")
			output.WriteString(warning)
			output.WriteByte('\n')
		}
	}
	return output.String()
}

func writeMetric(output *strings.Builder, metric aggregator.Metric) {
	output.WriteString(friendlyMetricName(metric.Name))
	output.WriteString(": ")
	if !metric.Available {
		output.WriteString("N/A | WoW N/A | 4wk avg N/A\n")
		return
	}
	output.WriteString(strconv.FormatInt(metric.Current, 10))
	output.WriteString(" | WoW ")
	output.WriteString(fmt.Sprintf("%+d (", metric.Comparison.Delta))
	if metric.Comparison.Percent == nil {
		output.WriteString("N/A")
	} else {
		output.WriteString(fmt.Sprintf("%+.1f%%", *metric.Comparison.Percent))
	}
	output.WriteString(") | 4wk avg ")
	output.WriteString(strconv.FormatFloat(metric.Average, 'f', 1, 64))
	output.WriteByte('\n')
}

func friendlyMetricName(name string) string {
	names := map[string]string{
		"new_users":              "New users",
		"onboarding_completions": "Onboarding completions",
		"total_expenses":         "Total expenses",
		"manual_expenses":        "Manual expenses",
		"correction_expenses":    "Correction expenses",
		"prorata_expenses":       "Pro-rata expenses",
		"active_expenses":        "Active expenses",
		"budget_periods":         "Budget periods",
		"tags":                   "Tags",
		"prorata_schedules":      "Pro-rata schedules",
		"completed_exports":      "Completed exports",
	}
	if label, ok := names[name]; ok {
		return label
	}
	return name
}

func reportRange(report aggregator.Report) (time.Time, time.Time) {
	if report.WindowSet == nil || report.WindowSet.GetReportWeek() == nil {
		return time.Time{}, time.Time{}
	}
	window := report.WindowSet.GetReportWeek()
	if window.GetStart() == nil || window.GetEnd() == nil {
		return time.Time{}, time.Time{}
	}
	return window.GetStart().AsTime().UTC(), window.GetEnd().AsTime().UTC()
}

func RuneCount(value string) int { return utf8.RuneCountInString(value) }

func ValidateRuneLimit(value string) error {
	if count := RuneCount(value); count > MaxReportRunes {
		return fmt.Errorf("report exceeds %d runes (%d)", MaxReportRunes, count)
	}
	return nil
}
