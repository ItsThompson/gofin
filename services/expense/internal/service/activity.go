package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/shared/reporting"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

// ActivityMetrics contains one typed result for each activity metric.
type ActivityMetrics struct {
	WindowSet          *reportingpb.ReportWindowSet
	TotalExpenses      *reportingpb.CountResult
	ManualExpenses     *reportingpb.CountResult
	CorrectionExpenses *reportingpb.CountResult
	ProrataExpenses    *reportingpb.CountResult
	ActiveExpenses     *reportingpb.CountResult
}

// ActivityService reads the global activity metrics independently from the
// expense lifecycle service.
type ActivityService struct {
	repo   repository.ActivityRepository
	logger *slog.Logger
}

// NewActivityService creates a reporting service with its narrow repository.
func NewActivityService(repo repository.ActivityRepository, logger ...*slog.Logger) *ActivityService {
	var serviceLogger *slog.Logger
	if len(logger) > 0 {
		serviceLogger = logger[0]
	}
	return &ActivityService{repo: repo, logger: serviceLogger}
}

// GetMetrics validates the shared report windows and reads each metric through
// its own repository query. A failed metric is represented as unavailable while
// successful metrics remain available in the same response.
func (s *ActivityService) GetMetrics(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*ActivityMetrics, error) {
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, fmt.Errorf("activity repository is not configured")
	}

	counts := &ActivityMetrics{WindowSet: windowSet}
	var err error

	counts.TotalExpenses, err = s.metricResult("total_expenses", ctx, windowSet, s.repo.CountTotal)
	if contextErr := activityContextError(ctx, err); contextErr != nil {
		return nil, contextErr
	}
	counts.ManualExpenses, err = s.metricResult("manual_expenses", ctx, windowSet, s.repo.CountManual)
	if contextErr := activityContextError(ctx, err); contextErr != nil {
		return nil, contextErr
	}
	counts.CorrectionExpenses, err = s.metricResult("correction_expenses", ctx, windowSet, s.repo.CountCorrections)
	if contextErr := activityContextError(ctx, err); contextErr != nil {
		return nil, contextErr
	}
	counts.ProrataExpenses, err = s.metricResult("prorata_expenses", ctx, windowSet, s.repo.CountProRata)
	if contextErr := activityContextError(ctx, err); contextErr != nil {
		return nil, contextErr
	}
	counts.ActiveExpenses, err = s.metricResult("active_expenses", ctx, windowSet, s.repo.CountActive)
	if contextErr := activityContextError(ctx, err); contextErr != nil {
		return nil, contextErr
	}

	return counts, nil
}

func (s *ActivityService) metricResult(
	metric string,
	ctx context.Context,
	windowSet *reportingpb.ReportWindowSet,
	count func(context.Context, *reportingpb.ReportWindowSet) (*repository.ActivityMetricCounts, error),
) (*reportingpb.CountResult, error) {
	counts, err := count(ctx, windowSet)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		s.logMetricUnavailable(metric, "dependency")
		return unavailableCountResult(), nil
	}
	if counts == nil {
		s.logMetricUnavailable(metric, "empty_result")
		return unavailableCountResult(), nil
	}
	return availableCountResult(counts), nil
}

func activityContextError(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return ctx.Err()
}

func availableCountResult(counts *repository.ActivityMetricCounts) *reportingpb.CountResult {
	return &reportingpb.CountResult{
		State: &reportingpb.CountResult_Available{
			Available: &reportingpb.CountValues{
				ReportWeek:             counts.ReportWeek,
				PreviousWeek:           counts.PreviousWeek,
				TrailingFourWeeksTotal: counts.TrailingFourWeeksTotal,
			},
		},
	}
}

func (s *ActivityService) logMetricUnavailable(metric, class string) {
	if s.logger == nil {
		return
	}
	s.logger.Warn("reporting metric unavailable",
		slog.String("service", "expense"),
		slog.String("rpc", "GetActivityMetrics"),
		slog.String("metric", metric),
		slog.String("code", "QUERY_FAILED"),
		slog.String("error_class", class),
	)
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
