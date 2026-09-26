package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetActiveExpensesByPeriodAfter_FiltersAndOrdersWithBoundedKeyset(t *testing.T) {
	rows := make([]*model.Expense, 0, 125)
	for i := 0; i < 123; i++ {
		row := buildTestExpense(fmt.Sprintf("period-%03d", i), "user-1", fmt.Sprintf("2026-05-01T00:00:%02dZ", i/3))
		row.ExpenseDateIso = fmt.Sprintf("2026-05-%02d", i%3+1)
		rows = append(rows, row)
	}
	corrected := buildTestExpense("corrected", "user-1", "2026-05-02T00:00:00Z")
	corrected.Status = "corrected"
	rows = append(rows, corrected)
	otherPeriod := buildTestExpense("other-period", "user-1", "2026-05-02T00:00:01Z")
	otherPeriod.PeriodMonth = 6
	rows = append(rows, otherPeriod)

	repo, client := newKeysetTestRepo(rows...)
	ctx := context.Background()
	var got []*model.Expense
	cursor := ActivePeriodCursor{}
	pages := 0
	for {
		page, next, hasMore, err := repo.GetActiveExpensesByPeriodAfter(ctx, "user-1", 2026, 5, cursor, 0)
		require.NoError(t, err)
		got = append(got, page...)
		pages++
		if !hasMore {
			break
		}
		cursor = next
		require.LessOrEqual(t, pages, 10)
	}

	require.Len(t, got, 123)
	for i, row := range got {
		assert.Equal(t, "active", row.Status)
		assert.Equal(t, int32(2026), row.PeriodYear)
		assert.Equal(t, int32(5), row.PeriodMonth)
		if i == 0 {
			continue
		}
		previous := got[i-1]
		assert.LessOrEqual(t, previous.ExpenseDateIso, row.ExpenseDateIso)
		if previous.ExpenseDateIso == row.ExpenseDateIso {
			assert.LessOrEqual(t, previous.CreatedAt, row.CreatedAt)
			if previous.CreatedAt == row.CreatedAt {
				assert.Less(t, previous.ID, row.ID)
			}
		}
	}

	assert.Equal(t, 3, pages)
	assert.Zero(t, client.countQueriesContaining("COUNT(*)"))
	assert.Zero(t, client.countQueriesContaining("OFFSET"))
	for _, query := range client.Queries() {
		if strings.Contains(query.SQL, "ORDER BY expense_date ASC") {
			assert.Equal(t, CompletePeriodPageSize+1, query.Params["limit"])
			assert.Contains(t, query.SQL, "status = 'active'")
			assert.Contains(t, query.SQL, "period_year = @year")
			assert.Contains(t, query.SQL, "period_month = @month")
		}
	}
}

func TestGetActiveExpensesByPeriodAfter_UsesTotalOrderCursor(t *testing.T) {
	const createdAt = "2026-05-01T00:00:00Z"
	repo, client := newKeysetTestRepo(
		buildTestExpense("period-c", "user-1", createdAt),
		buildTestExpense("period-a", "user-1", createdAt),
		buildTestExpense("period-b", "user-1", createdAt),
	)
	for _, row := range client.rows {
		row.ExpenseDateIso = "2026-05-01"
	}

	page, next, hasMore, err := repo.GetActiveExpensesByPeriodAfter(context.Background(), "user-1", 2026, 5, ActivePeriodCursor{}, 2)
	require.NoError(t, err)
	require.True(t, hasMore)
	assert.Equal(t, []string{"period-a", "period-b"}, []string{page[0].ID, page[1].ID})
	assert.Equal(t, ActivePeriodCursor{ExpenseDate: "2026-05-01", CreatedAt: createdAt, ID: "period-b"}, next)

	page, _, hasMore, err = repo.GetActiveExpensesByPeriodAfter(context.Background(), "user-1", 2026, 5, next, 2)
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Equal(t, []string{"period-c"}, []string{page[0].ID})

	queries := client.Queries()
	require.Len(t, queries, 2)
	assert.NotContains(t, queries[1].SQL, "(expense_date, created_at, id)")
	assert.Contains(t, queries[1].SQL, "expense_date = @cursor_expense_date")
	assert.Contains(t, queries[1].SQL, "created_at = @cursor_created_at")
	assert.Contains(t, queries[1].SQL, "id > @cursor_id")
}

func TestGetActiveExpensesByPeriodAfter_ClampsPageSizeToTransportBudget(t *testing.T) {
	repo, client := newKeysetTestRepo(buildTestExpense("period-1", "user-1", "2026-05-01T00:00:00Z"))

	_, _, _, err := repo.GetActiveExpensesByPeriodAfter(context.Background(), "user-1", 2026, 5, ActivePeriodCursor{}, CompletePeriodPageSize+1)
	require.NoError(t, err)
	assert.Equal(t, CompletePeriodPageSize+1, client.Queries()[0].Params["limit"])
}

func TestGetActiveExpensesByPeriodAfter_EmptyPeriodAndUserIsolation(t *testing.T) {
	repo, client := newKeysetTestRepo(
		buildTestExpense("other-user", "user-2", "2026-05-01T00:00:00Z"),
		buildTestExpense("other-period", "user-1", "2026-05-01T00:00:01Z"),
	)
	client.rows[1].PeriodMonth = 6

	rows, next, hasMore, err := repo.GetActiveExpensesByPeriodAfter(context.Background(), "user-1", 2026, 5, ActivePeriodCursor{}, 10)

	require.NoError(t, err)
	assert.Empty(t, rows)
	assert.Equal(t, ActivePeriodCursor{}, next)
	assert.False(t, hasMore)
	assert.Zero(t, client.countQueriesContaining("COUNT(*)"))
}

func TestGetActiveExpensesByPeriodAfter_DefaultsNonPositivePageSizes(t *testing.T) {
	for _, pageSize := range []int32{0, -1} {
		t.Run(fmt.Sprintf("page-size-%d", pageSize), func(t *testing.T) {
			repo, client := newKeysetTestRepo(buildTestExpense("period-1", "user-1", "2026-05-01T00:00:00Z"))

			_, _, _, err := repo.GetActiveExpensesByPeriodAfter(context.Background(), "user-1", 2026, 5, ActivePeriodCursor{}, pageSize)

			require.NoError(t, err)
			assert.Equal(t, CompletePeriodPageSize+1, client.Queries()[0].Params["limit"])
		})
	}
}

func TestGetActiveExpensesByPeriodAfter_RejectsPartialCursor(t *testing.T) {
	repo, client := newKeysetTestRepo()
	partialCursors := []ActivePeriodCursor{
		{ExpenseDate: "2026-05-01"},
		{ExpenseDate: "2026-05-01", CreatedAt: "2026-05-01T00:00:00Z"},
		{CreatedAt: "2026-05-01T00:00:00Z", ID: "period-1"},
	}

	for _, cursor := range partialCursors {
		_, _, _, err := repo.GetActiveExpensesByPeriodAfter(context.Background(), "user-1", 2026, 5, cursor, 10)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cursor must include")
	}
	assert.Empty(t, client.Queries())
}

type periodQueryErrorClient struct {
	*recordingImmudbClient
	err error
}

func (c *periodQueryErrorClient) SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	if strings.Contains(sql, "ORDER BY expense_date ASC") {
		c.record(sql, params)
		return nil, c.err
	}
	return c.recordingImmudbClient.SQLQuery(ctx, sql, params)
}

type periodMappingErrorClient struct {
	*recordingImmudbClient
}

func (c *periodMappingErrorClient) SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	if strings.Contains(sql, "ORDER BY expense_date ASC") {
		c.record(sql, params)
		return &SQLResult{Rows: []SQLRow{{Values: []SQLValue{fakeSQLValue{}}}}}, nil
	}
	return c.recordingImmudbClient.SQLQuery(ctx, sql, params)
}

func TestGetActiveExpensesByPeriodAfter_PropagatesQueryAndMappingErrors(t *testing.T) {
	queryClient := &periodQueryErrorClient{
		recordingImmudbClient: newRecordingImmudbClient(),
		err:                   errors.New("immudb unavailable"),
	}
	queryRepo := NewImmudbExpenseRepository(queryClient, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	_, _, _, err := queryRepo.GetActiveExpensesByPeriodAfter(context.Background(), "user-1", 2026, 5, ActivePeriodCursor{}, 10)
	require.Error(t, err)
	assert.ErrorContains(t, err, "querying active expenses for period")
	assert.ErrorContains(t, err, "immudb unavailable")

	mappingClient := &periodMappingErrorClient{recordingImmudbClient: newRecordingImmudbClient()}
	mappingRepo := NewImmudbExpenseRepository(mappingClient, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	_, _, _, err = mappingRepo.GetActiveExpensesByPeriodAfter(context.Background(), "user-1", 2026, 5, ActivePeriodCursor{}, 10)
	require.Error(t, err)
	assert.ErrorContains(t, err, "mapping active period expense row")
}
