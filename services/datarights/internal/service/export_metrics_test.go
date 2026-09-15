package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

type mockExportMetricsRepository struct {
	mock.Mock
}

func (m *mockExportMetricsRepository) CountCompletedExports(ctx context.Context, windows repository.ExportMetricsWindowSet) (repository.CompletedExportCounts, error) {
	args := m.Called(ctx, windows)
	if args.Get(0) == nil {
		return repository.CompletedExportCounts{}, args.Error(1)
	}
	return args.Get(0).(repository.CompletedExportCounts), args.Error(1)
}

var _ repository.ExportMetricsRepository = (*mockExportMetricsRepository)(nil)

func testWindowSet() (*reportingpb.ReportWindowSet, time.Time, time.Time, time.Time, time.Time, time.Time, time.Time) {
	reportStart := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	reportEnd := reportStart.Add(7 * 24 * time.Hour)
	previousStart := reportStart.Add(-7 * 24 * time.Hour)
	previousEnd := reportStart
	trailingStart := reportStart.Add(-28 * 24 * time.Hour)
	trailingEnd := reportStart
	return &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(reportStart),
		ReportWeek:               &reportingpb.UtcWindow{Start: timestamppb.New(reportStart), End: timestamppb.New(reportEnd)},
		PreviousWeek:             &reportingpb.UtcWindow{Start: timestamppb.New(previousStart), End: timestamppb.New(previousEnd)},
		TrailingFourWeeks:        &reportingpb.UtcWindow{Start: timestamppb.New(trailingStart), End: timestamppb.New(trailingEnd)},
	}, reportStart, reportEnd, previousStart, previousEnd, trailingStart, trailingEnd
}

func TestGetExportMetrics_ReturnsCountsForEachWindow(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	windowSet, reportStart, reportEnd, previousStart, previousEnd, trailingStart, trailingEnd := testWindowSet()
	repo.On("CountCompletedExports", mock.Anything, repository.ExportMetricsWindowSet{
		ReportWeekStart: reportStart, ReportWeekEnd: reportEnd,
		PreviousWeekStart: previousStart, PreviousWeekEnd: previousEnd,
		TrailingFourWeeksStart: trailingStart, TrailingFourWeeksEnd: trailingEnd,
	}).Return(repository.CompletedExportCounts{ReportWeek: 4, PreviousWeek: 2, TrailingFourWeeksTotal: 11}, nil)

	result, err := NewExportMetricsService(repo).GetExportMetrics(context.Background(), windowSet)

	require.NoError(t, err)
	available := result.GetAvailable()
	require.NotNil(t, available)
	assert.Equal(t, int64(4), available.ReportWeek)
	assert.Equal(t, int64(2), available.PreviousWeek)
	assert.Equal(t, int64(11), available.TrailingFourWeeksTotal)
	repo.AssertExpectations(t)
}

func TestGetExportMetrics_DBFailureMakesMetricUnavailable(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	windowSet, reportStart, reportEnd, previousStart, previousEnd, trailingStart, trailingEnd := testWindowSet()
	repo.On("CountCompletedExports", mock.Anything, repository.ExportMetricsWindowSet{
		ReportWeekStart: reportStart, ReportWeekEnd: reportEnd,
		PreviousWeekStart: previousStart, PreviousWeekEnd: previousEnd,
		TrailingFourWeeksStart: trailingStart, TrailingFourWeeksEnd: trailingEnd,
	}).Return(repository.CompletedExportCounts{}, errors.New("database unavailable"))

	result, err := NewExportMetricsService(repo).GetExportMetrics(context.Background(), windowSet)

	require.NoError(t, err)
	assert.Nil(t, result.GetAvailable())
	assert.Equal(t, reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED, result.GetUnavailable().GetCode())
	repo.AssertExpectations(t)
}

func TestGetExportMetrics_LogsSafeMetricFailure(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	windowSet, reportStart, reportEnd, previousStart, previousEnd, trailingStart, trailingEnd := testWindowSet()
	repo.On("CountCompletedExports", mock.Anything, repository.ExportMetricsWindowSet{
		ReportWeekStart: reportStart, ReportWeekEnd: reportEnd,
		PreviousWeekStart: previousStart, PreviousWeekEnd: previousEnd,
		TrailingFourWeeksStart: trailingStart, TrailingFourWeeksEnd: trailingEnd,
	}).Return(repository.CompletedExportCounts{}, errors.New("database unavailable"))
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	result, err := NewExportMetricsService(repo, logger).GetExportMetrics(context.Background(), windowSet)

	require.NoError(t, err)
	require.NotNil(t, result.GetUnavailable())
	var event map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &event))
	assert.Equal(t, "datarights", event["service"])
	assert.Equal(t, "GetExportMetrics", event["rpc"])
	assert.Equal(t, "completed_exports", event["metric"])
	assert.Equal(t, "QUERY_FAILED", event["code"])
	assert.Equal(t, "dependency", event["error_class"])
	assert.NotContains(t, logs.String(), "database unavailable")
	repo.AssertExpectations(t)
}

func TestGetExportMetrics_PreCanceledContextSkipsRepository(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	windowSet, _, _, _, _, _, _ := testWindowSet()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := NewExportMetricsService(repo).GetExportMetrics(ctx, windowSet)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
	repo.AssertNotCalled(t, "CountCompletedExports", mock.Anything, mock.Anything)
}

func TestGetExportMetrics_ContextCanceledAfterQueryDoesNotSucceed(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	windowSet, reportStart, reportEnd, previousStart, previousEnd, trailingStart, trailingEnd := testWindowSet()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo.On("CountCompletedExports", mock.Anything, repository.ExportMetricsWindowSet{
		ReportWeekStart: reportStart, ReportWeekEnd: reportEnd,
		PreviousWeekStart: previousStart, PreviousWeekEnd: previousEnd,
		TrailingFourWeeksStart: trailingStart, TrailingFourWeeksEnd: trailingEnd,
	}).Return(repository.CompletedExportCounts{ReportWeek: 4}, nil).Run(func(mock.Arguments) {
		cancel()
	})

	result, err := NewExportMetricsService(repo).GetExportMetrics(ctx, windowSet)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
	repo.AssertExpectations(t)
}

func TestGetExportMetrics_ContextErrorIsReturned(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	windowSet, reportStart, reportEnd, previousStart, previousEnd, trailingStart, trailingEnd := testWindowSet()
	repo.On("CountCompletedExports", mock.Anything, repository.ExportMetricsWindowSet{
		ReportWeekStart: reportStart, ReportWeekEnd: reportEnd,
		PreviousWeekStart: previousStart, PreviousWeekEnd: previousEnd,
		TrailingFourWeeksStart: trailingStart, TrailingFourWeeksEnd: trailingEnd,
	}).Return(repository.CompletedExportCounts{}, context.DeadlineExceeded)

	result, err := NewExportMetricsService(repo).GetExportMetrics(context.Background(), windowSet)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGetExportMetrics_RejectsIncompleteWindowSet(t *testing.T) {
	result, err := NewExportMetricsService(new(mockExportMetricsRepository)).GetExportMetrics(
		context.Background(), &reportingpb.ReportWindowSet{},
	)

	assert.Nil(t, result)
	assert.Error(t, err)
}
