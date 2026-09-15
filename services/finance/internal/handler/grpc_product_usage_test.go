package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/finance/internal/service"
	pb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

type productUsageMetricsServiceStub struct {
	called    bool
	windowSet *reportingpb.ReportWindowSet
	result    *service.ProductUsageMetrics
	err       error
}

func (s *productUsageMetricsServiceStub) GetProductUsageMetrics(_ context.Context, windowSet *reportingpb.ReportWindowSet) (*service.ProductUsageMetrics, error) {
	s.called = true
	s.windowSet = windowSet
	return s.result, s.err
}

func validHandlerProductUsageWindowSet() *reportingpb.ReportWindowSet {
	reportStart := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	return &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(reportStart),
		ReportWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(reportStart),
			End:   timestamppb.New(reportStart.Add(7 * 24 * time.Hour)),
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

func availableHandlerCountValues(reportWeek, previousWeek, trailing int64) *reportingpb.CountResult {
	return &reportingpb.CountResult{
		State: &reportingpb.CountResult_Available{
			Available: &reportingpb.CountValues{
				ReportWeek:             reportWeek,
				PreviousWeek:           previousWeek,
				TrailingFourWeeksTotal: trailing,
			},
		},
	}
}

func TestGetProductUsageMetricsValidatesThenInjectsService(t *testing.T) {
	windowSet := validHandlerProductUsageWindowSet()
	stub := &productUsageMetricsServiceStub{
		result: &service.ProductUsageMetrics{
			BudgetPeriods:    availableHandlerCountValues(1, 2, 3),
			Tags:             availableHandlerCountValues(4, 5, 6),
			ProrataSchedules: availableHandlerCountValues(7, 8, 9),
		},
	}
	handler := NewGRPCHandler(nil, stub)

	response, err := handler.GetProductUsageMetrics(context.Background(), &pb.GetProductUsageMetricsRequest{WindowSet: windowSet})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, stub.called)
	assert.Same(t, windowSet, stub.windowSet)
	assert.Same(t, windowSet, response.WindowSet)
	assert.Equal(t, int64(1), response.BudgetPeriods.GetAvailable().GetReportWeek())
	assert.Equal(t, int64(6), response.Tags.GetAvailable().GetTrailingFourWeeksTotal())
	assert.Equal(t, int64(8), response.ProrataSchedules.GetAvailable().GetPreviousWeek())
}

func TestGetProductUsageMetricsRejectsInvalidWindowBeforeService(t *testing.T) {
	stub := &productUsageMetricsServiceStub{}
	windowSet := validHandlerProductUsageWindowSet()
	windowSet.ReportWeek.End = timestamppb.New(windowSet.ReportWeek.Start.AsTime().Add(6 * 24 * time.Hour))
	handler := NewGRPCHandler(nil, stub)

	response, err := handler.GetProductUsageMetrics(context.Background(), &pb.GetProductUsageMetricsRequest{WindowSet: windowSet})

	assert.Nil(t, response)
	assert.False(t, stub.called)
	statusErr, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, statusErr.Code())
}

func TestGetProductUsageMetricsReturnsUnimplementedWhenServiceIsOmitted(t *testing.T) {
	handler := NewGRPCHandler(nil)

	response, err := handler.GetProductUsageMetrics(context.Background(), &pb.GetProductUsageMetricsRequest{WindowSet: validHandlerProductUsageWindowSet()})

	assert.Nil(t, response)
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestGetProductUsageMetricsMapsGroupContextErrors(t *testing.T) {
	stub := &productUsageMetricsServiceStub{err: context.DeadlineExceeded}
	handler := NewGRPCHandler(nil, stub)

	response, err := handler.GetProductUsageMetrics(context.Background(), &pb.GetProductUsageMetricsRequest{WindowSet: validHandlerProductUsageWindowSet()})

	assert.Nil(t, response)
	statusErr, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.DeadlineExceeded, statusErr.Code())
}

func TestGetProductUsageMetricsMapsUnexpectedServiceErrors(t *testing.T) {
	stub := &productUsageMetricsServiceStub{err: errors.New("transport failed")}
	handler := NewGRPCHandler(nil, stub)

	response, err := handler.GetProductUsageMetrics(context.Background(), &pb.GetProductUsageMetricsRequest{WindowSet: validHandlerProductUsageWindowSet()})

	assert.Nil(t, response)
	statusErr, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, statusErr.Code())
}

var _ ProductUsageMetricsService = (*productUsageMetricsServiceStub)(nil)
