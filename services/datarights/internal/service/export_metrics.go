package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	reporting "github.com/ItsThompson/gofin/services/shared/reporting"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

// ExportMetricsService reads aggregate export metrics for reporting windows.
type ExportMetricsService struct {
	repo   repository.ExportMetricsRepository
	logger *slog.Logger
}

// NewExportMetricsService creates a service backed by the export repository.
func NewExportMetricsService(repo repository.ExportMetricsRepository, logger ...*slog.Logger) *ExportMetricsService {
	var serviceLogger *slog.Logger
	if len(logger) > 0 {
		serviceLogger = logger[0]
	}
	return &ExportMetricsService{repo: repo, logger: serviceLogger}
}

// GetExportMetrics returns one count result for all requested windows.
// Database failures make this metric unavailable without failing the RPC.
func (s *ExportMetricsService) GetExportMetrics(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*reportingpb.CountResult, error) {
	startedAt := time.Now()
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, err
	}

	reportWeek := windowSet.GetReportWeek()
	previousWeek := windowSet.GetPreviousWeek()
	trailingFourWeeks := windowSet.GetTrailingFourWeeks()
	if err := ctx.Err(); err != nil {
		s.logMetricFailure("CANCELED", "canceled", time.Since(startedAt))
		return nil, err
	}
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
			s.logMetricFailure("CANCELED", "canceled", time.Since(startedAt))
			return nil, ctxErr
		}
		s.logMetricFailure("QUERY_FAILED", "dependency", time.Since(startedAt))
		return unavailableCountResult(), nil
	}
	if err := ctx.Err(); err != nil {
		s.logMetricFailure("CANCELED", "canceled", time.Since(startedAt))
		return nil, err
	}
	s.logMetricSuccess(time.Since(startedAt))

	return &reportingpb.CountResult{
		State: &reportingpb.CountResult_Available{Available: &reportingpb.CountValues{
			ReportWeek:             counts.ReportWeek,
			PreviousWeek:           counts.PreviousWeek,
			TrailingFourWeeksTotal: counts.TrailingFourWeeksTotal,
		}},
	}, nil
}

func (s *ExportMetricsService) logMetricSuccess(duration time.Duration) {
	if s.logger == nil {
		return
	}
	s.logger.Info("reporting metric query completed",
		slog.String("service", "datarights"),
		slog.String("rpc", "GetExportMetrics"),
		slog.String("metric", "completed_exports"),
		slog.String("outcome", "available"),
		slog.Duration("duration", duration),
	)
}

func (s *ExportMetricsService) logMetricFailure(code, class string, duration time.Duration) {
	if s.logger == nil {
		return
	}
	s.logger.Warn("reporting metric unavailable",
		slog.String("service", "datarights"),
		slog.String("rpc", "GetExportMetrics"),
		slog.String("metric", "completed_exports"),
		slog.String("code", code),
		slog.String("error_class", class),
		slog.Duration("duration", duration),
	)
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
