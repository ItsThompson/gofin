package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exportMetricsQueryer struct {
	query string
	args  []any
	row   pgx.Row
}

func (q *exportMetricsQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.query = query
	q.args = args
	return q.row
}

type exportMetricsRow struct {
	counts [3]int64
	err    error
}

func (r exportMetricsRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, count := range r.counts {
		*dest[i].(*int64) = count
	}
	return nil
}

func TestCountCompletedExports_UsesOneQueryAndThreeHalfOpenWindows(t *testing.T) {
	reportStart := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	reportEnd := reportStart.Add(7 * 24 * time.Hour)
	previousStart := reportStart.Add(-7 * 24 * time.Hour)
	trailingStart := reportStart.Add(-28 * 24 * time.Hour)
	windows := ExportMetricsWindowSet{
		ReportWeekStart:        reportStart,
		ReportWeekEnd:          reportEnd,
		PreviousWeekStart:      previousStart,
		PreviousWeekEnd:        reportStart,
		TrailingFourWeeksStart: trailingStart,
		TrailingFourWeeksEnd:   reportStart,
	}
	queryer := &exportMetricsQueryer{row: exportMetricsRow{counts: [3]int64{4, 2, 11}}}

	counts, err := countCompletedExports(context.Background(), queryer, windows)

	require.NoError(t, err)
	assert.Equal(t, CompletedExportCounts{ReportWeek: 4, PreviousWeek: 2, TrailingFourWeeksTotal: 11}, counts)
	assert.Contains(t, queryer.query, "status = $1")
	assert.Contains(t, queryer.query, "completed_at >= $6")
	assert.Contains(t, queryer.query, "completed_at < $3")
	assert.Contains(t, queryer.query, "COUNT(*) FILTER")
	assert.Equal(t, []any{
		"completed", reportStart, reportEnd, previousStart, reportStart, trailingStart, reportStart,
	}, queryer.args)
}

func TestCountCompletedExports_ReturnsDatabaseError(t *testing.T) {
	dbErr := errors.New("database unavailable")
	queryer := &exportMetricsQueryer{row: exportMetricsRow{err: dbErr}}

	counts, err := countCompletedExports(context.Background(), queryer, ExportMetricsWindowSet{})

	assert.Zero(t, counts)
	require.ErrorIs(t, err, dbErr)
}
