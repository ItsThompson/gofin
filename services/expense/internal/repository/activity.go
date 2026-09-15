package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ActivityMetricCounts contains counts for the three report windows.
type ActivityMetricCounts struct {
	ReportWeek             int64
	PreviousWeek           int64
	TrailingFourWeeksTotal int64
}

// The pinned immudb release does not support CASE aggregates, and grouped
// COUNT results do not reflect current rows. Count current created_at rows in Go.
const activityCountSelect = `SELECT created_at
	FROM expenses
	WHERE created_at >= @trailing_four_weeks_start
	AND created_at < @report_week_end
	AND status <> @redacted_status`

const (
	activityManualPredicate     = "AND (corrects_id IS NULL OR corrects_id = '') AND is_pro_rata = false"
	activityCorrectionPredicate = "AND corrects_id IS NOT NULL AND corrects_id <> ''"
	activityProRataPredicate    = "AND (corrects_id IS NULL OR corrects_id = '') AND is_pro_rata = true"
	activityActivePredicate     = "AND status = @active_status"
)

func (r *ImmudbExpenseRepository) CountTotal(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*ActivityMetricCounts, error) {
	return r.countActivityMetric(ctx, windowSet, "")
}

func (r *ImmudbExpenseRepository) CountManual(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*ActivityMetricCounts, error) {
	return r.countActivityMetric(ctx, windowSet, activityManualPredicate)
}

func (r *ImmudbExpenseRepository) CountCorrections(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*ActivityMetricCounts, error) {
	return r.countActivityMetric(ctx, windowSet, activityCorrectionPredicate)
}

func (r *ImmudbExpenseRepository) CountProRata(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*ActivityMetricCounts, error) {
	return r.countActivityMetric(ctx, windowSet, activityProRataPredicate)
}

func (r *ImmudbExpenseRepository) CountActive(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*ActivityMetricCounts, error) {
	return r.countActivityMetric(ctx, windowSet, activityActivePredicate)
}

func (r *ImmudbExpenseRepository) countActivityMetric(ctx context.Context, windowSet *reportingpb.ReportWindowSet, predicate string) (*ActivityMetricCounts, error) {
	if windowSet == nil || windowSet.GetReportWeek() == nil || windowSet.GetPreviousWeek() == nil || windowSet.GetTrailingFourWeeks() == nil {
		return nil, fmt.Errorf("activity report window set is incomplete")
	}

	reportWindow := windowSet.GetReportWeek()
	previousWindow := windowSet.GetPreviousWeek()
	trailingWindow := windowSet.GetTrailingFourWeeks()
	reportStart := formatActivityTimestamp(reportWindow.GetStart())
	reportEnd := formatActivityTimestamp(reportWindow.GetEnd())
	previousStart := formatActivityTimestamp(previousWindow.GetStart())
	previousEnd := formatActivityTimestamp(previousWindow.GetEnd())
	trailingStart := formatActivityTimestamp(trailingWindow.GetStart())
	trailingEnd := formatActivityTimestamp(trailingWindow.GetEnd())
	params := map[string]interface{}{
		"report_week_start":         reportStart,
		"report_week_end":           reportEnd,
		"previous_week_start":       previousStart,
		"previous_week_end":         previousEnd,
		"trailing_four_weeks_start": trailingStart,
		"trailing_four_weeks_end":   trailingEnd,
		"redacted_status":           model.StatusRedacted,
	}
	if predicate == activityActivePredicate {
		params["active_status"] = model.StatusActive
	}

	query := activityCountSelect + "\n\t" + predicate + ";"
	result, err := r.client.SQLQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("counting activity metric: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("activity metric count query returned no result")
	}

	counts := &ActivityMetricCounts{}
	for _, row := range result.Rows {
		if len(row.Values) < 1 {
			return nil, fmt.Errorf("activity metric count query returned an incomplete row")
		}
		createdAt := row.Values[0].GetString()
		if createdAt >= reportStart && createdAt < reportEnd {
			counts.ReportWeek++
		}
		if createdAt >= previousStart && createdAt < previousEnd {
			counts.PreviousWeek++
		}
		if createdAt >= trailingStart && createdAt < trailingEnd {
			counts.TrailingFourWeeksTotal++
		}
	}

	return counts, nil
}

func formatActivityTimestamp(timestamp *timestamppb.Timestamp) string {
	if timestamp == nil {
		return ""
	}
	return timestamp.AsTime().UTC().Format(time.RFC3339Nano)
}
