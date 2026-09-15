package handler

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockActivityService struct {
	mock.Mock
}

func (m *mockActivityService) GetMetrics(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*service.ActivityMetrics, error) {
	args := m.Called(ctx, windowSet)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.ActivityMetrics), args.Error(1)
}

func handlerActivityWindowSet() *reportingpb.ReportWindowSet {
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

func availableActivityCount(report, previous, trailing int64) *reportingpb.CountResult {
	return &reportingpb.CountResult{State: &reportingpb.CountResult_Available{
		Available: &reportingpb.CountValues{
			ReportWeek:             report,
			PreviousWeek:           previous,
			TrailingFourWeeksTotal: trailing,
		},
	}}
}

func TestGRPC_GetActivityMetrics_ValidatesAndUsesInjectedActivityService(t *testing.T) {
	activityService := new(mockActivityService)
	windows := handlerActivityWindowSet()
	ctx := context.Background()
	activityService.On("GetMetrics", ctx, windows).Return(&service.ActivityMetrics{
		WindowSet:          windows,
		TotalExpenses:      availableActivityCount(10, 8, 35),
		ManualExpenses:     availableActivityCount(6, 5, 20),
		CorrectionExpenses: availableActivityCount(2, 1, 8),
		ProrataExpenses:    availableActivityCount(2, 2, 7),
		ActiveExpenses:     availableActivityCount(9, 7, 30),
	}, nil)

	repo := new(mockExpenseRepository)
	expenseService := service.NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	handler := NewGRPCHandler(expenseService, activityService)

	response, err := handler.GetActivityMetrics(ctx, &pb.GetActivityMetricsRequest{WindowSet: windows})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Same(t, windows, response.GetWindowSet())
	require.NotNil(t, response.GetTotalExpenses().GetAvailable())
	require.NotNil(t, response.GetManualExpenses().GetAvailable())
	require.NotNil(t, response.GetCorrectionExpenses().GetAvailable())
	require.NotNil(t, response.GetProrataExpenses().GetAvailable())
	require.NotNil(t, response.GetActiveExpenses().GetAvailable())
	require.Equal(t, int64(10), response.GetTotalExpenses().GetAvailable().GetReportWeek())
	require.Equal(t, int64(5), response.GetManualExpenses().GetAvailable().GetPreviousWeek())
	activityService.AssertExpectations(t)
}

func TestGRPC_GetActivityMetrics_RejectsNilRequest(t *testing.T) {
	handler := NewGRPCHandler(nil, nil)

	response, err := handler.GetActivityMetrics(context.Background(), nil)

	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGRPC_GetActivityMetrics_RejectsInvalidWindowBeforeServiceCall(t *testing.T) {
	activityService := new(mockActivityService)
	repo := new(mockExpenseRepository)
	expenseService := service.NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	handler := NewGRPCHandler(expenseService, activityService)

	response, err := handler.GetActivityMetrics(context.Background(), &pb.GetActivityMetricsRequest{})

	require.Nil(t, response)
	require.Error(t, err)
	assertedStatus, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, assertedStatus.Code())
	require.Empty(t, activityService.ExpectedCalls)
}

func TestGRPC_GetActivityMetrics_MapsContextCancellation(t *testing.T) {
	activityService := new(mockActivityService)
	windows := handlerActivityWindowSet()
	ctx := context.Background()
	activityService.On("GetMetrics", ctx, windows).Return(nil, context.Canceled)

	repo := new(mockExpenseRepository)
	expenseService := service.NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	handler := NewGRPCHandler(expenseService, activityService)

	response, err := handler.GetActivityMetrics(ctx, &pb.GetActivityMetricsRequest{WindowSet: windows})

	require.Nil(t, response)
	require.Error(t, err)
	require.Equal(t, codes.Canceled, status.Code(err))
}

func TestGRPC_GetActivityMetrics_PreservesTypedUnavailableResults(t *testing.T) {
	activityService := new(mockActivityService)
	windows := handlerActivityWindowSet()
	ctx := context.Background()
	activityService.On("GetMetrics", ctx, windows).Return(&service.ActivityMetrics{
		WindowSet: windows,
		TotalExpenses: &reportingpb.CountResult{State: &reportingpb.CountResult_Unavailable{
			Unavailable: &reportingpb.MetricUnavailable{Code: reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED},
		}},
	}, nil)

	repo := new(mockExpenseRepository)
	expenseService := service.NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, time.Now, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	handler := NewGRPCHandler(expenseService, activityService)

	response, err := handler.GetActivityMetrics(ctx, &pb.GetActivityMetricsRequest{WindowSet: windows})

	require.NoError(t, err)
	require.NotNil(t, response.GetTotalExpenses().GetUnavailable())
	require.Equal(t, reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED, response.GetTotalExpenses().GetUnavailable().GetCode())
}

var _ ActivityService = (*mockActivityService)(nil)
