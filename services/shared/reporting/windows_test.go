package reporting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func validWindowSet() ReportWindowSet {
	reportStart := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	reportEnd := reportStart.Add(7 * 24 * time.Hour)
	return ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(reportStart),
		ReportWeek: &UtcWindow{
			Start: timestamppb.New(reportStart),
			End:   timestamppb.New(reportEnd),
		},
		PreviousWeek: &UtcWindow{
			Start: timestamppb.New(reportStart.Add(-7 * 24 * time.Hour)),
			End:   timestamppb.New(reportStart),
		},
		TrailingFourWeeks: &UtcWindow{
			Start: timestamppb.New(reportStart.Add(-28 * 24 * time.Hour)),
			End:   timestamppb.New(reportStart),
		},
	}
}

func TestValidateWindowSetAcceptsCanonicalUTCWindows(t *testing.T) {
	set := validWindowSet()
	require.NoError(t, ValidateWindowSet(&set))
}

func TestValidateWindowSetRejectsNilSet(t *testing.T) {
	var validationErr *WindowValidationError
	require.ErrorAs(t, ValidateWindowSet(nil), &validationErr)
	assert.Equal(t, MetricErrorCodeInvalidWindow, validationErr.Code)
	assert.Equal(t, "report_window_set", validationErr.Field)
}

func TestValidateWindowSetRejectsMissingValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ReportWindowSet)
	}{
		{
			name: "requested start",
			edit: func(set *ReportWindowSet) { set.RequestedReportWeekStart = nil },
		},
		{
			name: "report week",
			edit: func(set *ReportWindowSet) { set.ReportWeek = nil },
		},
		{
			name: "previous week",
			edit: func(set *ReportWindowSet) { set.PreviousWeek = nil },
		},
		{
			name: "trailing four weeks",
			edit: func(set *ReportWindowSet) { set.TrailingFourWeeks = nil },
		},
		{
			name: "report start timestamp",
			edit: func(set *ReportWindowSet) { set.ReportWeek.Start = nil },
		},
		{
			name: "report end timestamp",
			edit: func(set *ReportWindowSet) { set.ReportWeek.End = nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := validWindowSet()
			tt.edit(&set)

			var validationErr *WindowValidationError
			require.ErrorAs(t, ValidateWindowSet(&set), &validationErr)
			assert.Equal(t, MetricErrorCodeInvalidWindow, validationErr.Code)
		})
	}
}

func TestValidateWindowSetRejectsNonPositiveWindow(t *testing.T) {
	tests := []struct {
		name string
		end  time.Time
	}{
		{
			name: "zero duration",
			end:  time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "negative duration",
			end:  time.Date(2026, time.August, 2, 23, 59, 59, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := validWindowSet()
			set.ReportWeek.End = timestamppb.New(tt.end)
			require.Error(t, ValidateWindowSet(&set))
		})
	}
}

func TestValidateWindowSetRejectsReportWeekOutsideMondayMidnightUTC(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
	}{
		{
			name:  "not Monday",
			start: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "not midnight",
			start: time.Date(2026, time.August, 3, 1, 0, 0, 0, time.UTC),
		},
		{
			name:  "local offset is not UTC",
			start: time.Date(2026, time.August, 3, 0, 0, 0, 0, time.FixedZone("PDT", -7*60*60)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := validWindowSet()
			set.RequestedReportWeekStart = timestamppb.New(tt.start)
			set.ReportWeek.Start = timestamppb.New(tt.start)
			set.ReportWeek.End = timestamppb.New(tt.start.Add(7 * 24 * time.Hour))
			require.Error(t, ValidateWindowSet(&set))
		})
	}
}

func TestValidateWindowSetRejectsIncorrectReportRange(t *testing.T) {
	set := validWindowSet()
	set.ReportWeek.End = timestamppb.New(time.Date(2026, time.August, 10, 0, 0, 0, 1, time.UTC))

	var validationErr *WindowValidationError
	require.ErrorAs(t, ValidateWindowSet(&set), &validationErr)
	assert.Equal(t, "report_week", validationErr.Field)
}

func TestValidateWindowSetRejectsRequestedStartMismatch(t *testing.T) {
	set := validWindowSet()
	set.RequestedReportWeekStart = timestamppb.New(time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC))

	var validationErr *WindowValidationError
	require.ErrorAs(t, ValidateWindowSet(&set), &validationErr)
	assert.Equal(t, "requested_report_week_start", validationErr.Field)
}

func TestValidateWindowSetRejectsPreviousWeekRelationship(t *testing.T) {
	set := validWindowSet()
	set.PreviousWeek.Start = timestamppb.New(time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC))

	var validationErr *WindowValidationError
	require.ErrorAs(t, ValidateWindowSet(&set), &validationErr)
	assert.Equal(t, "previous_week", validationErr.Field)
}

func TestValidateWindowSetRejectsTrailingWeekRelationship(t *testing.T) {
	set := validWindowSet()
	set.TrailingFourWeeks.End = timestamppb.New(time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC))

	var validationErr *WindowValidationError
	require.ErrorAs(t, ValidateWindowSet(&set), &validationErr)
	assert.Equal(t, "trailing_four_weeks", validationErr.Field)
}

func TestValidateWindowSetRejectsMalformedTimestamp(t *testing.T) {
	set := validWindowSet()
	set.RequestedReportWeekStart = &timestamppb.Timestamp{Seconds: 1, Nanos: 1_000_000_000}

	var validationErr *WindowValidationError
	require.ErrorAs(t, ValidateWindowSet(&set), &validationErr)
	assert.Equal(t, MetricErrorCodeInvalidWindow, validationErr.Code)
	assert.NotEmpty(t, validationErr.Error())
}
