package repository

import (
	"context"

	"github.com/ItsThompson/gofin/services/finance/internal/db"
	"github.com/ItsThompson/gofin/services/shared/reporting"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"github.com/jackc/pgx/v5/pgtype"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// ProductUsageCounts holds one metric's counts for all report windows.
type ProductUsageCounts struct {
	ReportWeek             int64
	PreviousWeek           int64
	TrailingFourWeeksTotal int64
}

// ProductUsageRepository reads aggregate product-usage counts for the shared
// reporting windows. Each operation returns all windows from one query.
type ProductUsageRepository interface {
	CountBudgetPeriods(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (ProductUsageCounts, error)
	CountTags(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (ProductUsageCounts, error)
	CountProRataSchedules(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (ProductUsageCounts, error)
}

func productUsageTimestamps(windowSet *reportingpb.ReportWindowSet) ([6]pgtype.Timestamptz, error) {
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return [6]pgtype.Timestamptz{}, err
	}

	reportWeek := windowSet.GetReportWeek()
	previousWeek := windowSet.GetPreviousWeek()
	trailingFourWeeks := windowSet.GetTrailingFourWeeks()

	return [6]pgtype.Timestamptz{
		toProductUsageTimestamp(reportWeek.GetStart()),
		toProductUsageTimestamp(reportWeek.GetEnd()),
		toProductUsageTimestamp(previousWeek.GetStart()),
		toProductUsageTimestamp(previousWeek.GetEnd()),
		toProductUsageTimestamp(trailingFourWeeks.GetStart()),
		toProductUsageTimestamp(trailingFourWeeks.GetEnd()),
	}, nil
}

func toProductUsageTimestamp(timestamp *timestamppb.Timestamp) pgtype.Timestamptz {
	if timestamp == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: timestamp.AsTime(), Valid: true}
}

func countValuesFromBudgetPeriods(row db.CountBudgetPeriodsInWindowsRow) ProductUsageCounts {
	return ProductUsageCounts{
		ReportWeek:             row.ReportWeek,
		PreviousWeek:           row.PreviousWeek,
		TrailingFourWeeksTotal: row.TrailingFourWeeksTotal,
	}
}

func countValuesFromTags(row db.CountTagsInWindowsRow) ProductUsageCounts {
	return ProductUsageCounts{
		ReportWeek:             row.ReportWeek,
		PreviousWeek:           row.PreviousWeek,
		TrailingFourWeeksTotal: row.TrailingFourWeeksTotal,
	}
}

func countValuesFromProRataSchedules(row db.CountProRataSchedulesInWindowsRow) ProductUsageCounts {
	return ProductUsageCounts{
		ReportWeek:             row.ReportWeek,
		PreviousWeek:           row.PreviousWeek,
		TrailingFourWeeksTotal: row.TrailingFourWeeksTotal,
	}
}

func (r *PostgresFinanceRepository) CountBudgetPeriods(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (ProductUsageCounts, error) {
	timestamps, err := productUsageTimestamps(windowSet)
	if err != nil {
		return ProductUsageCounts{}, err
	}
	row, err := r.queries.CountBudgetPeriodsInWindows(ctx, db.CountBudgetPeriodsInWindowsParams{
		CreatedAt:   timestamps[0],
		CreatedAt_2: timestamps[1],
		CreatedAt_3: timestamps[2],
		CreatedAt_4: timestamps[3],
		CreatedAt_5: timestamps[4],
		CreatedAt_6: timestamps[5],
	})
	if err != nil {
		return ProductUsageCounts{}, err
	}
	return countValuesFromBudgetPeriods(row), nil
}

func (r *PostgresFinanceRepository) CountTags(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (ProductUsageCounts, error) {
	timestamps, err := productUsageTimestamps(windowSet)
	if err != nil {
		return ProductUsageCounts{}, err
	}
	row, err := r.queries.CountTagsInWindows(ctx, db.CountTagsInWindowsParams{
		CreatedAt:   timestamps[0],
		CreatedAt_2: timestamps[1],
		CreatedAt_3: timestamps[2],
		CreatedAt_4: timestamps[3],
		CreatedAt_5: timestamps[4],
		CreatedAt_6: timestamps[5],
	})
	if err != nil {
		return ProductUsageCounts{}, err
	}
	return countValuesFromTags(row), nil
}

func (r *PostgresFinanceRepository) CountProRataSchedules(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (ProductUsageCounts, error) {
	timestamps, err := productUsageTimestamps(windowSet)
	if err != nil {
		return ProductUsageCounts{}, err
	}
	row, err := r.queries.CountProRataSchedulesInWindows(ctx, db.CountProRataSchedulesInWindowsParams{
		CreatedAt:   timestamps[0],
		CreatedAt_2: timestamps[1],
		CreatedAt_3: timestamps[2],
		CreatedAt_4: timestamps[3],
		CreatedAt_5: timestamps[4],
		CreatedAt_6: timestamps[5],
	})
	if err != nil {
		return ProductUsageCounts{}, err
	}
	return countValuesFromProRataSchedules(row), nil
}
