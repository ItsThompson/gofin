package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/auth/internal/db"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

type growthCountRow struct {
	reportWeek             int64
	previousWeek           int64
	trailingFourWeeksTotal int64
	err                    error
}

func (r growthCountRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.reportWeek
	*dest[1].(*int64) = r.previousWeek
	*dest[2].(*int64) = r.trailingFourWeeksTotal
	return nil
}

func validRepositoryWindowSet() *reportingpb.ReportWindowSet {
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

func TestPostgresUserRepository_CountUsersCreatedUsesOneHalfOpenAggregateForAllRoles(t *testing.T) {
	set := validRepositoryWindowSet()
	var query string
	var args []interface{}

	fake := &fakeDBTX{
		queryRowFn: func(_ context.Context, sql string, queryArgs ...interface{}) pgx.Row {
			query = sql
			args = queryArgs
			return growthCountRow{reportWeek: 7, previousWeek: 3, trailingFourWeeksTotal: 10}
		},
	}
	repo := NewPostgresUserRepository(db.New(fake))

	counts, err := repo.CountUsersCreated(context.Background(), set)

	require.NoError(t, err)
	assert.Equal(t, GrowthCounts{ReportWeek: 7, PreviousWeek: 3, TrailingFourWeeksTotal: 10}, counts)
	assert.Contains(t, query, "created_at >= $5 AND created_at < $2")
	assert.Contains(t, query, "FILTER")
	assert.NotContains(t, strings.ToLower(query), "role")
	assert.Len(t, args, 6)
}

func TestPostgresUserRepository_CountOnboardingCompletionsUsesOneAggregate(t *testing.T) {
	set := validRepositoryWindowSet()
	var query string
	fake := &fakeDBTX{
		queryRowFn: func(_ context.Context, sql string, _ ...interface{}) pgx.Row {
			query = sql
			return growthCountRow{reportWeek: 4, previousWeek: 2, trailingFourWeeksTotal: 6}
		},
	}
	repo := NewPostgresUserRepository(db.New(fake))

	counts, err := repo.CountOnboardingCompletions(context.Background(), set)

	require.NoError(t, err)
	assert.Equal(t, GrowthCounts{ReportWeek: 4, PreviousWeek: 2, TrailingFourWeeksTotal: 6}, counts)
	assert.Contains(t, query, "onboarding_completed_at >= $5 AND onboarding_completed_at < $2")
	assert.Contains(t, query, "FILTER")
}
