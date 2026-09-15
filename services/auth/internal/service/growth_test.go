package service

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
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/auth/internal/repository"
	"github.com/ItsThompson/gofin/services/shared/reporting"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

type mockGrowthRepository struct {
	mock.Mock
}

func (m *mockGrowthRepository) CountUsersCreated(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (repository.GrowthCounts, error) {
	args := m.Called(ctx, windowSet)
	if args.Get(0) == nil {
		return repository.GrowthCounts{}, args.Error(1)
	}
	return args.Get(0).(repository.GrowthCounts), args.Error(1)
}

func (m *mockGrowthRepository) CountOnboardingCompletions(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (repository.GrowthCounts, error) {
	args := m.Called(ctx, windowSet)
	if args.Get(0) == nil {
		return repository.GrowthCounts{}, args.Error(1)
	}
	return args.Get(0).(repository.GrowthCounts), args.Error(1)
}

func newGrowthService(repo *mockGrowthRepository) *GrowthService {
	return NewGrowthService(repo, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

func validGrowthWindowSet() *reportingpb.ReportWindowSet {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	week := func(begin time.Time, days int) *reportingpb.UtcWindow {
		return &reportingpb.UtcWindow{
			Start: timestamppb.New(begin),
			End:   timestamppb.New(begin.Add(time.Duration(days) * 24 * time.Hour)),
		}
	}
	return &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(start),
		ReportWeek:               week(start, 7),
		PreviousWeek:             week(start.Add(-7*24*time.Hour), 7),
		TrailingFourWeeks:        week(start.Add(-28*24*time.Hour), 28),
	}
}

func setGrowthExpectations(repo *mockGrowthRepository, set *reportingpb.ReportWindowSet) {
	repo.On("CountUsersCreated", mock.Anything, set).Return(repository.GrowthCounts{
		ReportWeek: 1, PreviousWeek: 1, TrailingFourWeeksTotal: 1,
	}, nil)
	repo.On("CountOnboardingCompletions", mock.Anything, set).Return(repository.GrowthCounts{
		ReportWeek: 2, PreviousWeek: 2, TrailingFourWeeksTotal: 2,
	}, nil)
}

func TestGrowthService_ReturnsCountsForEachHalfOpenWindow(t *testing.T) {
	repo := new(mockGrowthRepository)
	set := validGrowthWindowSet()
	setGrowthExpectations(repo, set)

	result, err := newGrowthService(repo).GetGrowthMetrics(context.Background(), set)

	require.NoError(t, err)
	assert.Equal(t, &reportingpb.CountValues{ReportWeek: 1, PreviousWeek: 1, TrailingFourWeeksTotal: 1}, result.NewUsers.GetAvailable())
	assert.Equal(t, &reportingpb.CountValues{ReportWeek: 2, PreviousWeek: 2, TrailingFourWeeksTotal: 2}, result.OnboardingCompletions.GetAvailable())
	repo.AssertExpectations(t)
}

func TestGrowthService_IsolatesNewUsersQueryFailure(t *testing.T) {
	repo := new(mockGrowthRepository)
	set := validGrowthWindowSet()
	repo.On("CountUsersCreated", mock.Anything, set).Return(repository.GrowthCounts{}, errors.New("database unavailable"))
	repo.On("CountOnboardingCompletions", mock.Anything, set).Return(repository.GrowthCounts{
		ReportWeek: 4, PreviousWeek: 4, TrailingFourWeeksTotal: 4,
	}, nil)

	result, err := newGrowthService(repo).GetGrowthMetrics(context.Background(), set)

	require.NoError(t, err)
	assert.Equal(t, reporting.MetricErrorCodeQueryFailed, result.NewUsers.GetUnavailable().GetCode())
	assert.Equal(t, &reportingpb.CountValues{ReportWeek: 4, PreviousWeek: 4, TrailingFourWeeksTotal: 4}, result.OnboardingCompletions.GetAvailable())
}

func TestGrowthService_IsolatesOnboardingQueryFailure(t *testing.T) {
	repo := new(mockGrowthRepository)
	set := validGrowthWindowSet()
	repo.On("CountUsersCreated", mock.Anything, set).Return(repository.GrowthCounts{
		ReportWeek: 5, PreviousWeek: 5, TrailingFourWeeksTotal: 5,
	}, nil)
	repo.On("CountOnboardingCompletions", mock.Anything, set).Return(repository.GrowthCounts{}, errors.New("database unavailable"))

	result, err := newGrowthService(repo).GetGrowthMetrics(context.Background(), set)

	require.NoError(t, err)
	assert.Equal(t, &reportingpb.CountValues{ReportWeek: 5, PreviousWeek: 5, TrailingFourWeeksTotal: 5}, result.NewUsers.GetAvailable())
	assert.Equal(t, reporting.MetricErrorCodeQueryFailed, result.OnboardingCompletions.GetUnavailable().GetCode())
}

func TestGrowthService_ContextFailureIsGroupError(t *testing.T) {
	repo := new(mockGrowthRepository)
	set := validGrowthWindowSet()
	repo.On("CountUsersCreated", mock.Anything, set).Return(repository.GrowthCounts{}, context.DeadlineExceeded)

	result, err := newGrowthService(repo).GetGrowthMetrics(context.Background(), set)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGrowthService_InvalidWindowIsGroupError(t *testing.T) {
	repo := new(mockGrowthRepository)
	set := validGrowthWindowSet()
	set.ReportWeek.End = timestamppb.New(set.ReportWeek.Start.AsTime().Add(6 * 24 * time.Hour))

	result, err := newGrowthService(repo).GetGrowthMetrics(context.Background(), set)

	assert.Nil(t, result)
	var validationErr *reporting.WindowValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func TestGrowthService_PreCanceledContextIsGroupError(t *testing.T) {
	repo := new(mockGrowthRepository)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := newGrowthService(repo).GetGrowthMetrics(ctx, validGrowthWindowSet())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
	repo.AssertNotCalled(t, "CountUsersCreated", mock.Anything, mock.Anything)
}
