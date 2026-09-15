package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ItsThompson/gofin/services/auth/internal/db"
	"github.com/ItsThompson/gofin/services/shared/reporting"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

func (r *PostgresUserRepository) CountUsersCreated(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (GrowthCounts, error) {
	windows, err := growthWindowParams(windowSet)
	if err != nil {
		return GrowthCounts{}, err
	}

	row, err := r.queries.CountUsersCreatedInWindows(ctx, db.CountUsersCreatedInWindowsParams{
		CreatedAt:   timestamptz(windows.reportStart),
		CreatedAt_2: timestamptz(windows.reportEnd),
		CreatedAt_3: timestamptz(windows.previousStart),
		CreatedAt_4: timestamptz(windows.previousEnd),
		CreatedAt_5: timestamptz(windows.trailingStart),
		CreatedAt_6: timestamptz(windows.trailingEnd),
	})
	if err != nil {
		return GrowthCounts{}, err
	}
	return GrowthCounts{
		ReportWeek:             row.ReportWeek,
		PreviousWeek:           row.PreviousWeek,
		TrailingFourWeeksTotal: row.TrailingFourWeeksTotal,
	}, nil
}

func (r *PostgresUserRepository) CountOnboardingCompletions(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (GrowthCounts, error) {
	windows, err := growthWindowParams(windowSet)
	if err != nil {
		return GrowthCounts{}, err
	}

	row, err := r.queries.CountOnboardingCompletionsInWindows(ctx, db.CountOnboardingCompletionsInWindowsParams{
		OnboardingCompletedAt:   timestamptz(windows.reportStart),
		OnboardingCompletedAt_2: timestamptz(windows.reportEnd),
		OnboardingCompletedAt_3: timestamptz(windows.previousStart),
		OnboardingCompletedAt_4: timestamptz(windows.previousEnd),
		OnboardingCompletedAt_5: timestamptz(windows.trailingStart),
		OnboardingCompletedAt_6: timestamptz(windows.trailingEnd),
	})
	if err != nil {
		return GrowthCounts{}, err
	}
	return GrowthCounts{
		ReportWeek:             row.ReportWeek,
		PreviousWeek:           row.PreviousWeek,
		TrailingFourWeeksTotal: row.TrailingFourWeeksTotal,
	}, nil
}

type growthWindowParamsResult struct {
	reportStart   time.Time
	reportEnd     time.Time
	previousStart time.Time
	previousEnd   time.Time
	trailingStart time.Time
	trailingEnd   time.Time
}

func growthWindowParams(windowSet *reportingpb.ReportWindowSet) (growthWindowParamsResult, error) {
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return growthWindowParamsResult{}, err
	}
	return growthWindowParamsResult{
		reportStart:   windowSet.GetReportWeek().GetStart().AsTime().UTC(),
		reportEnd:     windowSet.GetReportWeek().GetEnd().AsTime().UTC(),
		previousStart: windowSet.GetPreviousWeek().GetStart().AsTime().UTC(),
		previousEnd:   windowSet.GetPreviousWeek().GetEnd().AsTime().UTC(),
		trailingStart: windowSet.GetTrailingFourWeeks().GetStart().AsTime().UTC(),
		trailingEnd:   windowSet.GetTrailingFourWeeks().GetEnd().AsTime().UTC(),
	}, nil
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
