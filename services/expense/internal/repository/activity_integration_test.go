//go:build integration

package repository

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testActivityWindows(reportStart time.Time) *reportingpb.ReportWindowSet {
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

func TestActivityMetrics_Integration_UsesCurrentRowsAndCreatedAtWindows(t *testing.T) {
	client := connectRealImmudb(t)
	repo := NewImmudbExpenseRepository(client, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := context.Background()
	require.NoError(t, repo.InitSchema(ctx))

	runID := strconv.FormatInt(time.Now().UnixNano(), 10)
	userID := "activity-" + runID
	mkID := func(s string) string { return runID + "-" + s }
	mkExpense := func(id, createdAt string, status string, correctsID string, isProRata bool) *model.Expense {
		expense := buildTestExpense(mkID(id), userID, createdAt)
		expense.Status = status
		expense.CorrectsID = correctsID
		expense.IsProRata = isProRata
		return expense
	}

	weekStart := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(time.Now().UnixNano()%300000)*7)
	rows := []*model.Expense{
		mkExpense("manual", weekStart.AddDate(0, 0, 1).Format(time.RFC3339), model.StatusActive, "", false),
		mkExpense("prorata", weekStart.AddDate(0, 0, -6).Format(time.RFC3339), model.StatusActive, "", true),
		mkExpense("correction", weekStart.AddDate(0, 0, 2).Format(time.RFC3339), model.StatusActive, mkID("manual"), true),
		mkExpense("old-correction", weekStart.AddDate(0, 0, -14).Format(time.RFC3339), model.StatusCorrected, mkID("prorata"), false),
		mkExpense("redacted", weekStart.AddDate(0, 0, 3).Format(time.RFC3339), model.StatusRedacted, "", false),
		mkExpense("null-parent", weekStart.AddDate(0, 0, -5).Format(time.RFC3339), model.StatusActive, "", false),
	}
	for _, row := range rows {
		_, err := repo.CreateExpense(ctx, row)
		require.NoError(t, err)
	}
	_, err := client.SQLExec(ctx, `UPDATE expenses SET corrects_id = NULL WHERE id = @id;`, map[string]interface{}{"id": mkID("null-parent")})
	require.NoError(t, err)
	windows := testActivityWindows(weekStart)

	total, err := repo.CountTotal(ctx, windows)
	require.NoError(t, err)
	assert.Equal(t, &ActivityMetricCounts{ReportWeek: 2, PreviousWeek: 2, TrailingFourWeeksTotal: 3}, total)

	manual, err := repo.CountManual(ctx, windows)
	require.NoError(t, err)
	assert.Equal(t, &ActivityMetricCounts{ReportWeek: 1, PreviousWeek: 1, TrailingFourWeeksTotal: 1}, manual)

	corrections, err := repo.CountCorrections(ctx, windows)
	require.NoError(t, err)
	assert.Equal(t, &ActivityMetricCounts{ReportWeek: 1, PreviousWeek: 0, TrailingFourWeeksTotal: 1}, corrections)

	prorata, err := repo.CountProRata(ctx, windows)
	require.NoError(t, err)
	assert.Equal(t, &ActivityMetricCounts{ReportWeek: 0, PreviousWeek: 1, TrailingFourWeeksTotal: 1}, prorata)

	active, err := repo.CountActive(ctx, windows)
	require.NoError(t, err)
	assert.Equal(t, &ActivityMetricCounts{ReportWeek: 2, PreviousWeek: 2, TrailingFourWeeksTotal: 2}, active)
}
