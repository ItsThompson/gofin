package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/auth/internal/repository"
	"github.com/ItsThompson/gofin/services/shared/reporting"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

// GrowthMetrics contains the two auth-owned count metrics for a report window
// set. Each metric can be unavailable without affecting its sibling.
type GrowthMetrics struct {
	WindowSet             *reportingpb.ReportWindowSet
	NewUsers              *reportingpb.CountResult
	OnboardingCompletions *reportingpb.CountResult
}

// GrowthService serves aggregate auth metrics for reporting.
type GrowthService struct {
	repo   repository.GrowthRepository
	logger *slog.Logger
}

// NewGrowthService creates a growth metrics service with injected storage.
func NewGrowthService(repo repository.GrowthRepository, logger *slog.Logger) *GrowthService {
	return &GrowthService{repo: repo, logger: logger}
}

// GetGrowthMetrics returns one result for each auth metric. Query failures are
// isolated to their metric, while cancellation, deadline, and unexpected
// panics fail the whole request.
func (s *GrowthService) GetGrowthMetrics(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (result *GrowthMetrics, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("growth metrics query failed")
		}
	}()

	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	newUsers := s.countUsers(ctx, windowSet)
	if newUsers.err != nil {
		return nil, newUsers.err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	onboarding := s.countOnboarding(ctx, windowSet)
	if onboarding.err != nil {
		return nil, onboarding.err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &GrowthMetrics{
		WindowSet:             windowSet,
		NewUsers:              newUsers.result,
		OnboardingCompletions: onboarding.result,
	}, nil
}

type metricCount struct {
	result *reportingpb.CountResult
	err    error
}

func (s *GrowthService) countUsers(ctx context.Context, windowSet *reportingpb.ReportWindowSet) metricCount {
	counts, err := s.repo.CountUsersCreated(ctx, windowSet)
	if err != nil {
		if isContextError(err) {
			return metricCount{err: err}
		}
		s.logMetricUnavailable("new_users")
		return metricCount{result: unavailableCountResult()}
	}
	return metricCount{result: availableCountResult(counts)}
}

func (s *GrowthService) countOnboarding(ctx context.Context, windowSet *reportingpb.ReportWindowSet) metricCount {
	counts, err := s.repo.CountOnboardingCompletions(ctx, windowSet)
	if err != nil {
		if isContextError(err) {
			return metricCount{err: err}
		}
		s.logMetricUnavailable("onboarding_completions")
		return metricCount{result: unavailableCountResult()}
	}
	return metricCount{result: availableCountResult(counts)}
}

func availableCountResult(counts repository.GrowthCounts) *reportingpb.CountResult {
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

func unavailableCountResult() *reportingpb.CountResult {
	return &reportingpb.CountResult{
		State: &reportingpb.CountResult_Unavailable{
			Unavailable: &reportingpb.MetricUnavailable{
				Code: reporting.MetricErrorCodeQueryFailed,
			},
		},
	}
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (s *GrowthService) logMetricUnavailable(metric string) {
	if s.logger == nil {
		return
	}
	s.logger.Warn("reporting metric unavailable",
		slog.String("service", "auth"),
		slog.String("rpc", "GetGrowthMetrics"),
		slog.String("metric", metric),
		slog.String("code", "QUERY_FAILED"),
		slog.String("error_class", "dependency"),
	)
}
