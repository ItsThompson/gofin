// Package reporting defines the shared reporting result types and validates
// the UTC windows used by reporting queries.
package reporting

import (
	"fmt"
	"time"

	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The aliases keep the generated protobuf contract available from the shared
// package while allowing callers to import the generated package directly when
// they need protobuf-specific APIs.
type UtcWindow = reportingpb.UtcWindow
type ReportWindowSet = reportingpb.ReportWindowSet
type CountValues = reportingpb.CountValues
type MetricUnavailable = reportingpb.MetricUnavailable
type CountResult = reportingpb.CountResult
type MetricErrorCode = reportingpb.MetricErrorCode

const (
	MetricErrorCodeUnspecified   = reportingpb.MetricErrorCode_METRIC_ERROR_CODE_UNSPECIFIED
	MetricErrorCodeQueryFailed   = reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED
	MetricErrorCodeInvalidWindow = reportingpb.MetricErrorCode_METRIC_ERROR_CODE_INVALID_WINDOW
	MetricErrorCodeInternal      = reportingpb.MetricErrorCode_METRIC_ERROR_CODE_INTERNAL
)

const (
	reportWeekDuration    = 7 * 24 * time.Hour
	trailingWeeksDuration = 4 * reportWeekDuration
)

// WindowValidationError identifies an invalid report window and carries the
// stable protocol error code used for unavailable metrics.
type WindowValidationError struct {
	Code    MetricErrorCode
	Field   string
	Message string
}

func (e *WindowValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateWindowSet verifies that all report windows are UTC, non-empty,
// aligned to Monday at midnight, and related to the requested report week.
// Protobuf timestamps represent instants, so validation evaluates each one in
// UTC. The ranges are start-inclusive and end-exclusive by contract.
func ValidateWindowSet(set *ReportWindowSet) error {
	if set == nil {
		return invalidWindow("report_window_set", "value is required")
	}

	requestedStart, err := validateTimestamp("requested_report_week_start", set.GetRequestedReportWeekStart())
	if err != nil {
		return err
	}

	if set.GetReportWeek() == nil {
		return invalidWindow("report_week", "window is required")
	}
	if set.GetPreviousWeek() == nil {
		return invalidWindow("previous_week", "window is required")
	}
	if set.GetTrailingFourWeeks() == nil {
		return invalidWindow("trailing_four_weeks", "window is required")
	}

	reportStart, reportEnd, err := validateWindow("report_week", set.GetReportWeek())
	if err != nil {
		return err
	}
	if reportStart.Weekday() != time.Monday || !isMidnight(reportStart) {
		return invalidWindow("report_week.start", "must be Monday at 00:00:00 UTC")
	}
	if reportEnd.Sub(reportStart) != reportWeekDuration {
		return invalidWindow("report_week", "must span exactly seven days")
	}

	if !requestedStart.Equal(reportStart) {
		return invalidWindow("requested_report_week_start", "must equal report_week.start")
	}

	previousStart, previousEnd, err := validateWindow("previous_week", set.GetPreviousWeek())
	if err != nil {
		return err
	}
	if previousEnd.Sub(previousStart) != reportWeekDuration {
		return invalidWindow("previous_week", "must span exactly seven days")
	}
	if !previousStart.Equal(reportStart.Add(-reportWeekDuration)) {
		return invalidWindow("previous_week.start", "must be seven days before report_week.start")
	}
	if !previousEnd.Equal(reportStart) {
		return invalidWindow("previous_week.end", "must equal report_week.start")
	}

	trailingStart, trailingEnd, err := validateWindow("trailing_four_weeks", set.GetTrailingFourWeeks())
	if err != nil {
		return err
	}
	if trailingEnd.Sub(trailingStart) != trailingWeeksDuration {
		return invalidWindow("trailing_four_weeks", "must span exactly 28 days")
	}
	if !trailingStart.Equal(reportStart.Add(-trailingWeeksDuration)) {
		return invalidWindow("trailing_four_weeks.start", "must be 28 days before report_week.start")
	}
	if !trailingEnd.Equal(reportStart) {
		return invalidWindow("trailing_four_weeks.end", "must equal report_week.start")
	}

	return nil
}

func validateWindow(field string, window *UtcWindow) (time.Time, time.Time, error) {
	start, err := validateTimestamp(field+".start", window.GetStart())
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := validateTimestamp(field+".end", window.GetEnd())
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, invalidWindow(field, "end must be after start")
	}
	return start, end, nil
}

func validateTimestamp(field string, timestamp *timestamppb.Timestamp) (time.Time, error) {
	if timestamp == nil {
		return time.Time{}, invalidWindow(field, "timestamp is required")
	}
	if err := timestamp.CheckValid(); err != nil {
		return time.Time{}, invalidWindow(field, err.Error())
	}
	return timestamp.AsTime().UTC(), nil
}

func isMidnight(value time.Time) bool {
	return value.Hour() == 0 && value.Minute() == 0 && value.Second() == 0 && value.Nanosecond() == 0
}

func invalidWindow(field, message string) error {
	return &WindowValidationError{
		Code:    MetricErrorCodeInvalidWindow,
		Field:   field,
		Message: message,
	}
}
