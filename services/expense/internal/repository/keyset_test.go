package repository

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

func newKeysetTestRepo(rows ...*model.Expense) (*ImmudbExpenseRepository, *recordingImmudbClient) {
	client := newRecordingImmudbClient(rows...)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewImmudbExpenseRepository(client, logger), client
}

// seedExportRows builds n canonical-UTC rows for a user, created one second
// apart so (created_at, id) ordering is unambiguous.
func seedExportRows(userID string, n int) []*model.Expense {
	rows := make([]*model.Expense, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("exp-%04d", i)
		createdAt := fmt.Sprintf("2026-05-01T00:%02d:%02dZ", i/60, i%60)
		rows = append(rows, buildTestExpense(id, userID, createdAt))
	}
	return rows
}

// walkFullExport pages through GetExpensesByUserAfter until hasMore is false and
// returns every row in order plus the number of repo calls (== pages).
func walkFullExport(t *testing.T, repo *ImmudbExpenseRepository, userID string, pageSize int32) ([]*model.Expense, int) {
	t.Helper()
	var all []*model.Expense
	cursor := ExpenseCursor{}
	calls := 0
	for {
		rows, next, hasMore, err := repo.GetExpensesByUserAfter(context.Background(), userID, cursor, pageSize)
		require.NoError(t, err)
		calls++
		all = append(all, rows...)
		if !hasMore {
			break
		}
		require.NotEqual(t, cursor, next, "cursor must advance between pages")
		cursor = next
		require.LessOrEqual(t, calls, 10_000, "keyset walk did not terminate")
	}
	return all, calls
}

func TestGetExpensesByUserAfter_UsesKeysetPredicateNotOffset(t *testing.T) {
	repo, client := newKeysetTestRepo(seedExportRows("user-1", 5)...)

	_, _, _, err := repo.GetExpensesByUserAfter(context.Background(), "user-1", ExpenseCursor{CreatedAt: "2026-05-01T00:00:01Z", ID: "exp-0001"}, 2)

	require.NoError(t, err)
	queries := client.Queries()
	require.Len(t, queries, 1)
	sql := queries[0].SQL
	assert.NotContains(t, strings.ToUpper(sql), "OFFSET")
	assert.NotContains(t, strings.ToUpper(sql), "COUNT(*)")
	assert.Contains(t, sql, "created_at > @cursor_created_at")
	assert.Contains(t, sql, "created_at = @cursor_created_at AND id > @cursor_id")
	assert.Contains(t, sql, "ORDER BY created_at ASC, id ASC")
	assert.Contains(t, sql, "LIMIT @limit")
	// pageSize+1 rows are fetched to derive hasMore without a COUNT.
	assert.Equal(t, int32(3), queries[0].Params["limit"])
	assert.Equal(t, "2026-05-01T00:00:01Z", queries[0].Params["cursor_created_at"])
	assert.Equal(t, "exp-0001", queries[0].Params["cursor_id"])
}

func TestGetExpensesByUserAfter_FirstPageOmitsCursorPredicate(t *testing.T) {
	repo, client := newKeysetTestRepo(seedExportRows("user-1", 3)...)

	_, _, _, err := repo.GetExpensesByUserAfter(context.Background(), "user-1", ExpenseCursor{}, 2)

	require.NoError(t, err)
	sql := client.Queries()[0].SQL
	assert.NotContains(t, sql, "@cursor_created_at")
	assert.NotContains(t, client.Queries()[0].Params, "cursor_created_at")
	assert.Contains(t, sql, "ORDER BY created_at ASC, id ASC")
}

func TestGetExpensesByUserAfter_DerivesHasMoreFromOverflowRow(t *testing.T) {
	repo, _ := newKeysetTestRepo(seedExportRows("user-1", 3)...)

	// First page: 2 of 3 rows, more remain.
	page1, next1, hasMore1, err := repo.GetExpensesByUserAfter(context.Background(), "user-1", ExpenseCursor{}, 2)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.True(t, hasMore1)
	assert.Equal(t, "exp-0001", next1.ID)
	assert.Equal(t, page1[1].CreatedAt, next1.CreatedAt)

	// Second page: the final row, no overflow, hasMore false.
	page2, _, hasMore2, err := repo.GetExpensesByUserAfter(context.Background(), "user-1", next1, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.False(t, hasMore2)
	assert.Equal(t, "exp-0002", page2[0].ID)
}

func TestGetExpensesByUserAfter_ExactPageBoundaryReportsNoMore(t *testing.T) {
	// Exactly pageSize rows: the pageSize+1 fetch returns no overflow row.
	repo, _ := newKeysetTestRepo(seedExportRows("user-1", 2)...)

	rows, _, hasMore, err := repo.GetExpensesByUserAfter(context.Background(), "user-1", ExpenseCursor{}, 2)

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.False(t, hasMore)
}

func TestGetExpensesByUserAfter_OrdersBySharedCreatedAtWithIDTiebreaker(t *testing.T) {
	// Three rows share a created_at; the id tiebreaker must give a stable,
	// total order and a correct keyset boundary across the shared second.
	const sharedTime = "2026-05-01T00:00:00Z"
	repo, _ := newKeysetTestRepo(
		buildTestExpense("exp-c", "user-1", sharedTime),
		buildTestExpense("exp-a", "user-1", sharedTime),
		buildTestExpense("exp-b", "user-1", sharedTime),
	)

	page1, next1, hasMore1, err := repo.GetExpensesByUserAfter(context.Background(), "user-1", ExpenseCursor{}, 2)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.True(t, hasMore1)
	assert.Equal(t, "exp-a", page1[0].ID)
	assert.Equal(t, "exp-b", page1[1].ID)
	assert.Equal(t, ExpenseCursor{CreatedAt: sharedTime, ID: "exp-b"}, next1)

	// Seeking past (sharedTime, exp-b) must return only exp-c, not re-emit
	// exp-a/exp-b that share the same created_at.
	page2, _, hasMore2, err := repo.GetExpensesByUserAfter(context.Background(), "user-1", next1, 2)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.False(t, hasMore2)
	assert.Equal(t, "exp-c", page2[0].ID)
}

func TestGetExpensesByUserAfter_EmptyResultCleanTermination(t *testing.T) {
	repo, client := newKeysetTestRepo()

	rows, next, hasMore, err := repo.GetExpensesByUserAfter(context.Background(), "user-empty", ExpenseCursor{}, 50)

	require.NoError(t, err)
	assert.Empty(t, rows)
	assert.False(t, hasMore)
	assert.Equal(t, ExpenseCursor{}, next)
	assert.Zero(t, client.countQueriesContaining("COUNT(*)"))
	assert.Zero(t, client.countQueriesContaining("OFFSET"))
}

func TestGetExpensesByUserAfter_DefaultsPageSizeWhenNonPositive(t *testing.T) {
	repo, client := newKeysetTestRepo(seedExportRows("user-1", 1)...)

	_, _, _, err := repo.GetExpensesByUserAfter(context.Background(), "user-1", ExpenseCursor{}, 0)

	require.NoError(t, err)
	// Default page size + 1 overflow row.
	assert.Equal(t, DefaultStreamPageSize+1, client.Queries()[0].Params["limit"])
}

func TestGetExpensesByUserAfter_FullExportNoOffsetNoPerPageCount(t *testing.T) {
	const pageSize = int32(10)
	repo, client := newKeysetTestRepo(seedExportRows("user-1", 55)...)

	rows, calls := walkFullExport(t, repo, "user-1", pageSize)

	require.Len(t, rows, 55)
	// 55 rows / 10 per page = 6 pages (5 full + 1 partial).
	assert.Equal(t, 6, calls)
	assert.Zero(t, client.countQueriesContaining("OFFSET"), "keyset export must never use OFFSET")
	assert.LessOrEqual(t, client.countQueriesContaining("COUNT(*)"), 1, "at most one COUNT(*) per full export")
	// Exactly one data query per page, no extra scans.
	assert.Equal(t, calls, len(client.Queries()))
}

func TestGetExpensesByUserAfter_QueryCountScalesLinearly(t *testing.T) {
	const pageSize = int32(10)
	for _, pages := range []int{1, 10, 50, 100} {
		t.Run(fmt.Sprintf("P=%d", pages), func(t *testing.T) {
			repo, client := newKeysetTestRepo(seedExportRows("user-1", pages*int(pageSize))...)

			rows, calls := walkFullExport(t, repo, "user-1", pageSize)

			require.Len(t, rows, pages*int(pageSize))
			// One data query per page: total queries grow linearly (O(P)), never
			// quadratically, and no page re-scans a prior page (no OFFSET).
			assert.Equal(t, pages, calls)
			assert.Equal(t, pages, len(client.Queries()))
			assert.Zero(t, client.countQueriesContaining("OFFSET"))
		})
	}
}
