package service

import (
	"context"
	"errors"

	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	reporting "github.com/ItsThompson/gofin/services/shared/reporting"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

// ExportMetricsService reads aggregate export metrics for reporting windows.
type ExportMetricsService struct {
	repo repository.ExportMetricsRepository
}

// NewExportMetricsService creates a service backed by the export repository.
func NewExportMetricsService(repo repository.ExportMetricsRepository) *ExportMetricsService {
	return &ExportMetricsService{repo: repo}
}

// GetExportMetrics returns one count result for all requested windows.
// Database failures make this metric unavailable without failing the RPC.
func (s *ExportMetricsService) GetExportMetrics(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*reportingpb.CountResult, error) {
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, err
	}

	reportWeek := windowSet.GetReportWeek()
	previousWeek := windowSet.GetPreviousWeek()
	trailingFourWeeks := windowSet.GetTrailingFourWeeks()
	counts, err := s.repo.CountCompletedExports(ctx, repository.ExportMetricsWindowSet{
		ReportWeekStart:        reportWeek.GetStart().AsTime(),
		ReportWeekEnd:          reportWeek.GetEnd().AsTime(),
		PreviousWeekStart:      previousWeek.GetStart().AsTime(),
		PreviousWeekEnd:        previousWeek.GetEnd().AsTime(),
		TrailingFourWeeksStart: trailingFourWeeks.GetStart().AsTime(),
		TrailingFourWeeksEnd:   trailingFourWeeks.GetEnd().AsTime(),
	})
	if err != nil {
		if ctxErr := contextError(ctx, err); ctxErr != nil {
			return nil, ctxErr
		}
		return unavailableCountResult(), nil
	}

	return &reportingpb.CountResult{
		State: &reportingpb.CountResult_Available{Available: &reportingpb.CountValues{
			ReportWeek:             counts.ReportWeek,
			PreviousWeek:           counts.PreviousWeek,
			TrailingFourWeeksTotal: counts.TrailingFourWeeksTotal,
		}},
	}, nil
}

func contextError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return nil
}

func unavailableCountResult() *reportingpb.CountResult {
	return &reportingpb.CountResult{
		State: &reportingpb.CountResult_Unavailable{
			Unavailable: &reportingpb.MetricUnavailable{
				Code: reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED,
			},
		},
	}
}
