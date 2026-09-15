package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func validActivityWindowSet() *reportingpb.ReportWindowSet {
	reportStart := time.Date(2026, time.May, 4, 0, 0, 0, 0, time.UTC)
	return &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(reportStart),
		ReportWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart),
			End:   timestamppb.New(reportStart.AddDate(0, 0, 7)),
		},
		PreviousWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart.AddDate(0, 0, -7)),
			End:   timestamppb.New(reportStart),
		},
		TrailingFourWeeks: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart.AddDate(0, 0, -28)),
			End:   timestamppb.New(reportStart),
		},
	}
}

func activityMetricCounts(report, previous, trailing int64) *repository.ActivityMetricCounts {
	return &repository.ActivityMetricCounts{
		ReportWeek:             report,
		PreviousWeek:           previous,
		TrailingFourWeeksTotal: trailing,
	}
}

type mockActivityRepository struct {
	mock.Mock
}

func (m *mockActivityRepository) CountTotal(ctx context.Context, windows *reportingpb.ReportWindowSet) (*repository.ActivityMetricCounts, error) {
	args := m.Called(ctx, windows)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ActivityMetricCounts), args.Error(1)
}

func (m *mockActivityRepository) CountManual(ctx context.Context, windows *reportingpb.ReportWindowSet) (*repository.ActivityMetricCounts, error) {
	args := m.Called(ctx, windows)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ActivityMetricCounts), args.Error(1)
}

func (m *mockActivityRepository) CountCorrections(ctx context.Context, windows *reportingpb.ReportWindowSet) (*repository.ActivityMetricCounts, error) {
	args := m.Called(ctx, windows)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ActivityMetricCounts), args.Error(1)
}

func (m *mockActivityRepository) CountProRata(ctx context.Context, windows *reportingpb.ReportWindowSet) (*repository.ActivityMetricCounts, error) {
	args := m.Called(ctx, windows)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ActivityMetricCounts), args.Error(1)
}

func (m *mockActivityRepository) CountActive(ctx context.Context, windows *reportingpb.ReportWindowSet) (*repository.ActivityMetricCounts, error) {
	args := m.Called(ctx, windows)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ActivityMetricCounts), args.Error(1)
}

func TestGetMetrics_ReturnsAvailableCountsForEachMetric(t *testing.T) {
	repo := new(mockActivityRepository)
	windows := validActivityWindowSet()
	ctx := context.Background()
	repo.On("CountTotal", ctx, windows).Return(activityMetricCounts(10, 8, 35), nil)
	repo.On("CountManual", ctx, windows).Return(activityMetricCounts(6, 5, 20), nil)
	repo.On("CountCorrections", ctx, windows).Return(activityMetricCounts(2, 1, 8), nil)
	repo.On("CountProRata", ctx, windows).Return(activityMetricCounts(2, 2, 7), nil)
	repo.On("CountActive", ctx, windows).Return(activityMetricCounts(9, 7, 30), nil)
	svc := NewActivityService(repo)

	metrics, err := svc.GetMetrics(ctx, windows)

	require.NoError(t, err)
	require.NotNil(t, metrics)
	assert.Same(t, windows, metrics.WindowSet)
	assert.Equal(t, int64(10), metrics.TotalExpenses.GetAvailable().GetReportWeek())
	assert.Equal(t, int64(5), metrics.ManualExpenses.GetAvailable().GetPreviousWeek())
	assert.Equal(t, int64(8), metrics.CorrectionExpenses.GetAvailable().GetTrailingFourWeeksTotal())
	assert.Equal(t, int64(2), metrics.ProrataExpenses.GetAvailable().GetReportWeek())
	assert.Equal(t, int64(9), metrics.ActiveExpenses.GetAvailable().GetReportWeek())
	repo.AssertExpectations(t)
}

func TestGetMetrics_IsolatesMetricQueryFailureAsUnavailable(t *testing.T) {
	repo := new(mockActivityRepository)
	windows := validActivityWindowSet()
	ctx := context.Background()
	repo.On("CountTotal", ctx, windows).Return(activityMetricCounts(10, 8, 35), nil)
	repo.On("CountManual", ctx, windows).Return(nil, errors.New("manual query failed"))
	repo.On("CountCorrections", ctx, windows).Return(activityMetricCounts(2, 1, 8), nil)
	repo.On("CountProRata", ctx, windows).Return(activityMetricCounts(2, 2, 7), nil)
	repo.On("CountActive", ctx, windows).Return(activityMetricCounts(9, 7, 30), nil)
	svc := NewActivityService(repo)

	metrics, err := svc.GetMetrics(ctx, windows)

	require.NoError(t, err)
	require.NotNil(t, metrics)
	require.NotNil(t, metrics.ManualExpenses.GetUnavailable())
	assert.Equal(t, reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED, metrics.ManualExpenses.GetUnavailable().GetCode())
	assert.NotNil(t, metrics.TotalExpenses.GetAvailable())
	assert.NotNil(t, metrics.CorrectionExpenses.GetAvailable())
	assert.NotNil(t, metrics.ProrataExpenses.GetAvailable())
	assert.NotNil(t, metrics.ActiveExpenses.GetAvailable())
	repo.AssertExpectations(t)
}

func TestGetMetrics_RejectsInvalidWindowSetBeforeQuerying(t *testing.T) {
	repo := new(mockActivityRepository)
	svc := NewActivityService(repo)

	metrics, err := svc.GetMetrics(context.Background(), nil)

	require.Error(t, err)
	assert.Nil(t, metrics)
	assert.Empty(t, repo.ExpectedCalls)
}

func TestGetMetrics_ReturnsContextFailureInsteadOfUnavailable(t *testing.T) {
	repo := new(mockActivityRepository)
	windows := validActivityWindowSet()
	ctx := context.Background()
	repo.On("CountTotal", ctx, windows).Return(nil, context.Canceled)
	svc := NewActivityService(repo)

	metrics, err := svc.GetMetrics(ctx, windows)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, metrics)
}

func TestGetMetrics_PreCanceledContextDoesNotQuery(t *testing.T) {
	repo := new(mockActivityRepository)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := NewActivityService(repo)

	metrics, err := svc.GetMetrics(ctx, validActivityWindowSet())

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, metrics)
	repo.AssertNotCalled(t, "CountTotal", mock.Anything, mock.Anything)
}
