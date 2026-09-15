package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/finance/internal/db"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

func TestProductUsageTimestampsMapAllWindowBounds(t *testing.T) {
	reportStart := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	windowSet := &reportingpb.ReportWindowSet{
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

	got, err := productUsageTimestamps(windowSet)

	require.NoError(t, err)
	require.Len(t, got, 6)
	assert.Equal(t, reportStart, got[0].Time)
	assert.Equal(t, reportStart.Add(7*24*time.Hour), got[1].Time)
	assert.Equal(t, reportStart.Add(-7*24*time.Hour), got[2].Time)
	assert.Equal(t, reportStart, got[3].Time)
	assert.Equal(t, reportStart.Add(-28*24*time.Hour), got[4].Time)
	assert.Equal(t, reportStart, got[5].Time)
	for _, timestamp := range got {
		assert.True(t, timestamp.Valid)
	}
}

func TestProductUsageTimestampsReturnInvalidValuesForNilSet(t *testing.T) {
	got, err := productUsageTimestamps(nil)

	assert.Error(t, err)
	for _, timestamp := range got {
		assert.False(t, timestamp.Valid)
	}
}

type productUsageCountRow struct {
	reportWeek             int64
	previousWeek           int64
	trailingFourWeeksTotal int64
}

func (r productUsageCountRow) Scan(dest ...interface{}) error {
	*dest[0].(*int64) = r.reportWeek
	*dest[1].(*int64) = r.previousWeek
	*dest[2].(*int64) = r.trailingFourWeeksTotal
	return nil
}

type productUsageDBTX struct {
	queryRowFn func(context.Context, string, ...interface{}) pgx.Row
}

func (db *productUsageDBTX) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (*productUsageDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query call")
}

func (db *productUsageDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return db.queryRowFn(ctx, sql, args...)
}

func TestProductUsageQueriesUseOneAggregateAndAllStatuses(t *testing.T) {
	set := &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)),
		ReportWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)),
			End:   timestamppb.New(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)),
		},
		PreviousWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)),
			End:   timestamppb.New(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)),
		},
		TrailingFourWeeks: &reportingpb.UtcWindow{
			Start: timestamppb.New(time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)),
			End:   timestamppb.New(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)),
		},
	}

	queries := []struct {
		name string
		call func(*PostgresFinanceRepository) (ProductUsageCounts, error)
		want ProductUsageCounts
	}{
		{
			name: "budget periods",
			call: func(repo *PostgresFinanceRepository) (ProductUsageCounts, error) {
				return repo.CountBudgetPeriods(context.Background(), set)
			},
			want: ProductUsageCounts{ReportWeek: 1, PreviousWeek: 2, TrailingFourWeeksTotal: 3},
		},
		{
			name: "tags",
			call: func(repo *PostgresFinanceRepository) (ProductUsageCounts, error) {
				return repo.CountTags(context.Background(), set)
			},
			want: ProductUsageCounts{ReportWeek: 4, PreviousWeek: 5, TrailingFourWeeksTotal: 6},
		},
		{
			name: "pro-rata schedules",
			call: func(repo *PostgresFinanceRepository) (ProductUsageCounts, error) {
				return repo.CountProRataSchedules(context.Background(), set)
			},
			want: ProductUsageCounts{ReportWeek: 7, PreviousWeek: 8, TrailingFourWeeksTotal: 9},
		},
	}

	for _, test := range queries {
		t.Run(test.name, func(t *testing.T) {
			var query string
			var args []interface{}
			fake := &productUsageDBTX{queryRowFn: func(_ context.Context, sql string, queryArgs ...interface{}) pgx.Row {
				query = sql
				args = queryArgs
				return productUsageCountRow{reportWeek: test.want.ReportWeek, previousWeek: test.want.PreviousWeek, trailingFourWeeksTotal: test.want.TrailingFourWeeksTotal}
			}}
			repo := NewPostgresFinanceRepository(db.New(fake))

			got, err := test.call(repo)

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
			assert.Contains(t, query, "created_at >= $5 AND created_at < $2")
			assert.Contains(t, query, "FILTER")
			assert.NotContains(t, strings.ToLower(query), "status")
			assert.Len(t, args, 6)
		})
	}
}
