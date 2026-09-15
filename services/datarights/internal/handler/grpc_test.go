package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
	pb "github.com/ItsThompson/gofin/services/datarights/proto/datarightspb"
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

func validExportMetricsRequest() (*pb.GetExportMetricsRequest, repository.ExportMetricsWindowSet) {
	reportStart := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	reportEnd := reportStart.Add(7 * 24 * time.Hour)
	previousStart := reportStart.Add(-7 * 24 * time.Hour)
	trailingStart := reportStart.Add(-28 * 24 * time.Hour)
	windowSet := &pb.GetExportMetricsRequest{WindowSet: &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(reportStart),
		ReportWeek:               &reportingpb.UtcWindow{Start: timestamppb.New(reportStart), End: timestamppb.New(reportEnd)},
		PreviousWeek:             &reportingpb.UtcWindow{Start: timestamppb.New(previousStart), End: timestamppb.New(reportStart)},
		TrailingFourWeeks:        &reportingpb.UtcWindow{Start: timestamppb.New(trailingStart), End: timestamppb.New(reportStart)},
	}}
	return windowSet, repository.ExportMetricsWindowSet{
		ReportWeekStart:        reportStart,
		ReportWeekEnd:          reportEnd,
		PreviousWeekStart:      previousStart,
		PreviousWeekEnd:        reportStart,
		TrailingFourWeeksStart: trailingStart,
		TrailingFourWeeksEnd:   reportStart,
	}
}

func TestGetExportMetrics_ReturnsCountsAndEchoesWindowSet(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	req, windows := validExportMetricsRequest()
	repo.On("CountCompletedExports", mock.Anything, windows).
		Return(repository.CompletedExportCounts{ReportWeek: 4, PreviousWeek: 2, TrailingFourWeeksTotal: 11}, nil)
	handler := NewGRPCHandler(service.NewExportMetricsService(repo))

	resp, err := handler.GetExportMetrics(context.Background(), req)

	require.NoError(t, err)
	assert.Same(t, req.WindowSet, resp.WindowSet)
	assert.Equal(t, int64(4), resp.CompletedExports.GetAvailable().GetReportWeek())
	assert.Equal(t, int64(2), resp.CompletedExports.GetAvailable().GetPreviousWeek())
	assert.Equal(t, int64(11), resp.CompletedExports.GetAvailable().GetTrailingFourWeeksTotal())
	repo.AssertExpectations(t)
}

func TestGetExportMetrics_QueryFailureReturnsUnavailableResult(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	req, windows := validExportMetricsRequest()
	repo.On("CountCompletedExports", mock.Anything, windows).
		Return(repository.CompletedExportCounts{}, errors.New("database unavailable"))
	handler := NewGRPCHandler(service.NewExportMetricsService(repo))

	resp, err := handler.GetExportMetrics(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp.CompletedExports.GetUnavailable())
	assert.Equal(t, int32(1), int32(resp.CompletedExports.GetUnavailable().GetCode()))
	assert.Same(t, req.WindowSet, resp.WindowSet)
}

func TestGetExportMetrics_NilRequestReturnsInvalidArgument(t *testing.T) {
	handler := NewGRPCHandler(nil)

	resp, err := handler.GetExportMetrics(context.Background(), nil)

	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetExportMetrics_InvalidWindowReturnsInvalidArgument(t *testing.T) {
	handler := NewGRPCHandler(nil)

	resp, err := handler.GetExportMetrics(context.Background(), &pb.GetExportMetricsRequest{})

	assert.Nil(t, resp)
	statusErr, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, statusErr.Code())
}

func TestGetExportMetrics_ContextErrorReturnsContextStatus(t *testing.T) {
	repo := new(mockExportMetricsRepository)
	req, windows := validExportMetricsRequest()
	repo.On("CountCompletedExports", mock.Anything, windows).
		Return(repository.CompletedExportCounts{}, context.DeadlineExceeded)
	handler := NewGRPCHandler(service.NewExportMetricsService(repo))

	resp, err := handler.GetExportMetrics(context.Background(), req)

	assert.Nil(t, resp)
	statusErr, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.DeadlineExceeded, statusErr.Code())
}
