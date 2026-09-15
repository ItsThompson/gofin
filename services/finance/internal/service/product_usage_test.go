package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

type productUsageRepositoryStub struct {
	budgetPeriodsFn    func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error)
	tagsFn             func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error)
	prorataSchedulesFn func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error)
}

func (s *productUsageRepositoryStub) CountBudgetPeriods(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
	return s.budgetPeriodsFn(ctx, windowSet)
}

func (s *productUsageRepositoryStub) CountTags(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
	return s.tagsFn(ctx, windowSet)
}

func (s *productUsageRepositoryStub) CountProRataSchedules(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
	return s.prorataSchedulesFn(ctx, windowSet)
}

func validProductUsageWindowSet() *reportingpb.ReportWindowSet {
	reportStart := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	reportEnd := reportStart.Add(7 * 24 * time.Hour)
	return &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(reportStart),
		ReportWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart),
			End:   timestamppb.New(reportEnd),
		},
		PreviousWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart.Add(-7 * 24 * time.Hour)),
			End:   timestamppb.New(reportStart),
		},
		TrailingFourWeeks: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart.Add(-28 * 24 * time.Hour)),
			End:   timestamppb.New(reportStart),
		},
	}
}

func productUsageCounts(reportWeek, previousWeek, trailing int64) repository.ProductUsageCounts {
	return repository.ProductUsageCounts{
		ReportWeek:             reportWeek,
		PreviousWeek:           previousWeek,
		TrailingFourWeeksTotal: trailing,
	}
}

func TestProductUsageServiceReturnsTypedCounts(t *testing.T) {
	windowSet := validProductUsageWindowSet()
	var budgetWindowSet, tagsWindowSet, prorataWindowSet *reportingpb.ReportWindowSet
	repo := &productUsageRepositoryStub{
		budgetPeriodsFn: func(_ context.Context, got *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			budgetWindowSet = got
			return productUsageCounts(1, 2, 3), nil
		},
		tagsFn: func(_ context.Context, got *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			tagsWindowSet = got
			return productUsageCounts(4, 5, 6), nil
		},
		prorataSchedulesFn: func(_ context.Context, got *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			prorataWindowSet = got
			return productUsageCounts(7, 8, 9), nil
		},
	}

	result, err := NewProductUsageService(repo).GetProductUsageMetrics(context.Background(), windowSet)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Same(t, windowSet, budgetWindowSet)
	assert.Same(t, windowSet, tagsWindowSet)
	assert.Same(t, windowSet, prorataWindowSet)

	budget := result.BudgetPeriods.GetAvailable()
	require.NotNil(t, budget)
	assert.Equal(t, int64(1), budget.GetReportWeek())
	assert.Equal(t, int64(3), budget.GetTrailingFourWeeksTotal())
	tags := result.Tags.GetAvailable()
	require.NotNil(t, tags)
	assert.Equal(t, int64(4), tags.GetReportWeek())
	assert.Equal(t, int64(6), tags.GetTrailingFourWeeksTotal())
	prorata := result.ProrataSchedules.GetAvailable()
	require.NotNil(t, prorata)
	assert.Equal(t, int64(8), prorata.GetPreviousWeek())
	assert.Equal(t, int64(9), prorata.GetTrailingFourWeeksTotal())
}

func TestProductUsageServiceIsolatesMetricQueryFailures(t *testing.T) {
	queryErr := errors.New("database unavailable")
	repo := &productUsageRepositoryStub{
		budgetPeriodsFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return repository.ProductUsageCounts{}, queryErr
		},
		tagsFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(4, 5, 6), nil
		},
		prorataSchedulesFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(7, 8, 9), nil
		},
	}

	result, err := NewProductUsageService(repo).GetProductUsageMetrics(context.Background(), validProductUsageWindowSet())

	require.NoError(t, err)
	require.NotNil(t, result.BudgetPeriods)
	require.NotNil(t, result.BudgetPeriods.GetUnavailable())
	assert.Equal(t, reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED, result.BudgetPeriods.GetUnavailable().GetCode())
	assert.Equal(t, int64(4), result.Tags.GetAvailable().GetReportWeek())
	assert.Equal(t, int64(7), result.ProrataSchedules.GetAvailable().GetReportWeek())
}

func TestProductUsageServiceRejectsInvalidWindowSetBeforeQueries(t *testing.T) {
	called := false
	repo := &productUsageRepositoryStub{
		budgetPeriodsFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			called = true
			return productUsageCounts(1, 1, 1), nil
		},
		tagsFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(1, 1, 1), nil
		},
		prorataSchedulesFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(1, 1, 1), nil
		},
	}
	windowSet := validProductUsageWindowSet()
	windowSet.ReportWeek.End = timestamppb.New(windowSet.ReportWeek.Start.AsTime().Add(6 * 24 * time.Hour))

	result, err := NewProductUsageService(repo).GetProductUsageMetrics(context.Background(), windowSet)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.False(t, called)
}

func TestProductUsageServiceAbortsOnMetricPanic(t *testing.T) {
	repo := &productUsageRepositoryStub{
		budgetPeriodsFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			panic("unexpected repository panic")
		},
		tagsFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(1, 1, 1), nil
		},
		prorataSchedulesFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(1, 1, 1), nil
		},
	}

	result, err := NewProductUsageService(repo).GetProductUsageMetrics(context.Background(), validProductUsageWindowSet())

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "budget_periods")
}

func TestProductUsageServiceReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := &productUsageRepositoryStub{
		budgetPeriodsFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(1, 1, 1), nil
		},
		tagsFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(1, 1, 1), nil
		},
		prorataSchedulesFn: func(context.Context, *reportingpb.ReportWindowSet) (repository.ProductUsageCounts, error) {
			return productUsageCounts(1, 1, 1), nil
		},
	}

	result, err := NewProductUsageService(repo).GetProductUsageMetrics(ctx, validProductUsageWindowSet())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
}

var _ repository.ProductUsageRepository = (*productUsageRepositoryStub)(nil)
