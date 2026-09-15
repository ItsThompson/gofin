package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/auth/internal/repository"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
	pb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/shared/reporting"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

type growthRepositoryStub struct {
	users      repository.GrowthCounts
	onboarding repository.GrowthCounts
	usersErr   error
	onboardErr error
}

func (s *growthRepositoryStub) CountUsersCreated(context.Context, *reportingpb.ReportWindowSet) (repository.GrowthCounts, error) {
	return s.users, s.usersErr
}

func (s *growthRepositoryStub) CountOnboardingCompletions(context.Context, *reportingpb.ReportWindowSet) (repository.GrowthCounts, error) {
	return s.onboarding, s.onboardErr
}

func handlerGrowthWindowSet() *reportingpb.ReportWindowSet {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	window := func(begin time.Time, days int) *reportingpb.UtcWindow {
		return &reportingpb.UtcWindow{
			Start: timestamppb.New(begin),
			End:   timestamppb.New(begin.Add(time.Duration(days) * 24 * time.Hour)),
		}
	}
	return &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(start),
		ReportWeek:               window(start, 7),
		PreviousWeek:             window(start.Add(-7*24*time.Hour), 7),
		TrailingFourWeeks:        window(start.Add(-28*24*time.Hour), 28),
	}
}

func newGrowthAdapter(repo repository.GrowthRepository) *GrowthGRPCAdapter {
	return NewGrowthGRPCAdapter(service.NewGrowthService(repo, slog.New(slog.NewJSONHandler(io.Discard, nil))))
}

func TestGrowthGRPCAdapter_ReturnsCountsAndEchoesWindows(t *testing.T) {
	adapter := newGrowthAdapter(&growthRepositoryStub{
		users:      repository.GrowthCounts{ReportWeek: 7, PreviousWeek: 3, TrailingFourWeeksTotal: 10},
		onboarding: repository.GrowthCounts{ReportWeek: 4, PreviousWeek: 2, TrailingFourWeeksTotal: 6},
	})
	windowSet := handlerGrowthWindowSet()

	response, err := adapter.GetGrowthMetrics(context.Background(), &pb.GetGrowthMetricsRequest{WindowSet: windowSet})

	require.NoError(t, err)
	assert.Equal(t, windowSet, response.GetWindowSet())
	assert.Equal(t, int64(7), response.GetNewUsers().GetAvailable().GetReportWeek())
	assert.Equal(t, int64(6), response.GetOnboardingCompletions().GetAvailable().GetTrailingFourWeeksTotal())
}

func TestGrowthGRPCAdapter_IsolatesMetricQueryFailure(t *testing.T) {
	adapter := newGrowthAdapter(&growthRepositoryStub{
		users:      repository.GrowthCounts{},
		usersErr:   errors.New("query failed"),
		onboarding: repository.GrowthCounts{ReportWeek: 4, PreviousWeek: 2, TrailingFourWeeksTotal: 6},
	})

	response, err := adapter.GetGrowthMetrics(context.Background(), &pb.GetGrowthMetricsRequest{WindowSet: handlerGrowthWindowSet()})

	require.NoError(t, err)
	assert.Equal(t, reporting.MetricErrorCodeQueryFailed, response.GetNewUsers().GetUnavailable().GetCode())
	assert.Equal(t, int64(4), response.GetOnboardingCompletions().GetAvailable().GetReportWeek())
}

func TestGrowthGRPCAdapter_RejectsNilOrInvalidWindows(t *testing.T) {
	adapter := newGrowthAdapter(&growthRepositoryStub{})

	_, err := adapter.GetGrowthMetrics(context.Background(), nil)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	invalid := handlerGrowthWindowSet()
	invalid.ReportWeek.End = timestamppb.New(invalid.ReportWeek.Start.AsTime().Add(6 * 24 * time.Hour))
	_, err = adapter.GetGrowthMetrics(context.Background(), &pb.GetGrowthMetricsRequest{WindowSet: invalid})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

type growthRPCStub struct {
	mock.Mock
}

func (s *growthRPCStub) GetGrowthMetrics(ctx context.Context, req *pb.GetGrowthMetricsRequest) (*pb.GrowthMetricsResponse, error) {
	args := s.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.GrowthMetricsResponse), args.Error(1)
}

func TestGRPCHandler_DelegatesGrowthMetricsToInjectedAdapter(t *testing.T) {
	stub := new(growthRPCStub)
	request := &pb.GetGrowthMetricsRequest{WindowSet: handlerGrowthWindowSet()}
	response := &pb.GrowthMetricsResponse{WindowSet: request.WindowSet}
	stub.On("GetGrowthMetrics", mock.Anything, request).Return(response, nil)

	handler := NewGRPCHandler(nil, slog.New(slog.NewJSONHandler(io.Discard, nil)), stub)
	actual, err := handler.GetGrowthMetrics(context.Background(), request)

	require.NoError(t, err)
	assert.Same(t, response, actual)
	stub.AssertExpectations(t)
}
