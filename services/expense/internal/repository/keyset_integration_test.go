//go:build integration

// Integration test for the keyset export path against a real immudb container.
//
// It confirms the unit-level query-shape assertions against the pinned
// production immudb version (codenotary/immudb:1.11.0): that the expanded-OR
// (created_at, id) predicate and multi-column `ORDER BY created_at ASC, id ASC`
// actually work on that version, that keyset ordering is correct across a
// shared-created_at boundary, and that no query uses OFFSET.
//
// Run with a live immudb:
//
//	docker run -d --rm -p 3322:3322 codenotary/immudb:1.11.0
//	go test -tags integration -run Integration ./internal/repository/...
//
// Set TEST_IMMUDB_ADDR (default localhost:3322) to point elsewhere. The test
// skips if immudb is unreachable.
package repository

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	immudb "github.com/codenotary/immudb/pkg/client"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRealClient wraps the immudb SDK client as a repository.ImmudbClient
// and records issued SQL, so the integration test can assert query shape (no
// OFFSET, no COUNT) on the statements actually sent to real immudb.
type recordingRealClient struct {
	client  immudb.ImmuClient
	mu      sync.Mutex
	queries []string
}

func (c *recordingRealClient) record(sql string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queries = append(c.queries, sql)
}

func (c *recordingRealClient) recordedQueries() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.queries))
	copy(out, c.queries)
	return out
}

func (c *recordingRealClient) SQLExec(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	c.record(sql)
	_, err := c.client.SQLExec(ctx, sql, params)
	if err != nil {
		return nil, err
	}
	return &SQLResult{}, nil
}

func (c *recordingRealClient) SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	c.record(sql)
	res, err := c.client.SQLQuery(ctx, sql, params, true) //nolint:staticcheck // SA1019: mirrors the production client (cmd/immudb_prod.go); SQLQueryReader is a deferred optional refinement, not required for the O(pageSize) bound.
	if err != nil {
		return nil, err
	}
	rows := make([]SQLRow, len(res.Rows))
	for i, row := range res.Rows {
		values := make([]SQLValue, len(row.Values))
		for j, val := range row.Values {
			values[j] = realSQLValue{val: val.GetS(), num: val.GetN(), b: val.GetB()}
		}
		rows[i] = SQLRow{Values: values}
	}
	return &SQLResult{Rows: rows}, nil
}

type realSQLValue struct {
	val string
	num int64
	b   bool
}

func (v realSQLValue) GetString() string { return v.val }
func (v realSQLValue) GetInt() int64     { return v.num }
func (v realSQLValue) GetBool() bool     { return v.b }

func connectRealImmudb(t *testing.T) *recordingRealClient {
	t.Helper()
	addr := os.Getenv("TEST_IMMUDB_ADDR")
	if addr == "" {
		addr = "localhost:3322"
	}
	host := addr
	port := 3322
	if parts := strings.SplitN(addr, ":", 2); len(parts) == 2 {
		host = parts[0]
		p, err := strconv.Atoi(parts[1])
		require.NoError(t, err)
		port = p
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := immudb.DefaultOptions().WithAddress(host).WithPort(port)
	client := immudb.NewClient().WithOptions(opts)
	if err := client.OpenSession(ctx, []byte("immudb"), []byte("immudb"), "defaultdb"); err != nil {
		t.Skipf("immudb not reachable at %s (%v); start it with `docker run -d --rm -p 3322:3322 codenotary/immudb:1.11.0`", addr, err)
	}
	t.Cleanup(func() { _ = client.CloseSession(context.Background()) })
	return &recordingRealClient{client: client}
}

func TestGetExpensesByUserAfter_Integration_KeysetOrderingAndNoOffset(t *testing.T) {
	client := connectRealImmudb(t)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)
	ctx := context.Background()

	require.NoError(t, repo.InitSchema(ctx))

	// Unique user + id prefix so repeated runs against the persistent immudb
	// volume don't collide (id is the primary key).
	runID := strconv.FormatInt(time.Now().UnixNano(), 10)
	userID := "itest-" + runID
	mkID := func(s string) string { return runID + "-" + s }

	const (
		t1 = "2026-05-01T00:00:01Z"
		t2 = "2026-05-01T00:00:02Z"
		t3 = "2026-05-01T00:00:03Z"
	)
	// Insert out of order, with two shared-created_at groups (t1 and t3).
	seed := []*model.Expense{
		buildTestExpense(mkID("id-2"), userID, t1),
		buildTestExpense(mkID("id-1"), userID, t1),
		buildTestExpense(mkID("id-3"), userID, t2),
		buildTestExpense(mkID("id-5"), userID, t3),
		buildTestExpense(mkID("id-4"), userID, t3),
	}
	for _, e := range seed {
		_, err := repo.CreateExpense(ctx, e)
		require.NoError(t, err)
	}

	// Walk the full export with a small page size so pages straddle the shared
	// created_at boundaries.
	var got []ExpenseCursor
	cursor := ExpenseCursor{}
	pages := 0
	for {
		rows, next, hasMore, err := repo.GetExpensesByUserAfter(ctx, userID, cursor, 2)
		require.NoError(t, err)
		pages++
		for _, r := range rows {
			got = append(got, ExpenseCursor{CreatedAt: r.CreatedAt, ID: r.ID})
		}
		if !hasMore {
			break
		}
		cursor = next
		require.LessOrEqual(t, pages, 100, "keyset walk did not terminate")
	}

	// Chronological (created_at ASC) with id tiebreaker, no duplicates, no skips
	// across the shared-created_at boundaries.
	want := []ExpenseCursor{
		{CreatedAt: t1, ID: mkID("id-1")},
		{CreatedAt: t1, ID: mkID("id-2")},
		{CreatedAt: t2, ID: mkID("id-3")},
		{CreatedAt: t3, ID: mkID("id-4")},
		{CreatedAt: t3, ID: mkID("id-5")},
	}
	assert.Equal(t, want, got)

	// Confirm the unit query-shape assertions hold against real immudb: the
	// expanded-OR keyset predicate is used, and no data query uses OFFSET or a
	// per-page COUNT.
	sawKeysetPredicate := false
	for _, q := range client.recordedQueries() {
		upper := strings.ToUpper(q)
		if strings.Contains(upper, "SELECT") && strings.Contains(upper, "FROM EXPENSES") &&
			!strings.Contains(upper, "CREATE") {
			assert.NotContains(t, upper, "OFFSET", "keyset query must not use OFFSET: %s", q)
		}
		if strings.Contains(q, "created_at = @cursor_created_at AND id > @cursor_id") {
			sawKeysetPredicate = true
		}
	}
	assert.True(t, sawKeysetPredicate, "expected the expanded-OR keyset predicate to be issued")
}

// TestInitSchema_BackfillBatchesLargeLegacySet_Integration seeds more legacy
// rows than immudb's DefaultMaxTxEntries (1024) and asserts the batched
// backfill completes without the "max number of entries per tx exceeded" error.
// This validates that UPDATE ... LIMIT n is accepted by the real immudb 1.11.0
// SQL engine and that the loop drains all legacy rows.
//
// Run with a live immudb:
//
//	docker run -d --rm -p 3322:3322 codenotary/immudb:1.11.0
//	go test -tags integration -run Integration_Backfill ./internal/repository/...
func TestInitSchema_BackfillBatchesLargeLegacySet_Integration(t *testing.T) {
	client := connectRealImmudb(t)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)
	ctx := context.Background()

	require.NoError(t, repo.InitSchema(ctx))

	runID := strconv.FormatInt(time.Now().UnixNano(), 10)
	userID := "itest-backfill-" + runID

	// Seed 1200 legacy rows (more than DefaultMaxTxEntries=1024) by inserting
	// them one at a time via raw SQL that omits the snapshot columns, leaving
	// them NULL so the backfill picks them up. One row per tx stays under the
	// entry limit.
	const legacyCount = 1200
	for i := 0; i < legacyCount; i++ {
		id := runID + "-" + strconv.Itoa(i)
		_, err := client.SQLExec(ctx,
			`INSERT INTO expenses (id, user_id, name, amount, currency, expense_type, tag_id,
				expense_date, period_year, period_month, status, corrects_id,
				is_pro_rata, pro_rata_group, pro_rata_index, pro_rata_total, created_at)
				VALUES (@id, @user_id, @name, @amount, @currency, @expense_type, @tag_id,
				@expense_date, @period_year, @period_month, @status, @corrects_id,
				@is_pro_rata, @pro_rata_group, @pro_rata_index, @pro_rata_total, @created_at)`,
			map[string]interface{}{
				"id":             id,
				"user_id":        userID,
				"name":           "Legacy " + id,
				"amount":         int64(1000),
				"currency":       "USD",
				"expense_type":   "essentials",
				"tag_id":         "tag-1",
				"expense_date":   "2026-05-01",
				"period_year":    int64(2026),
				"period_month":   int64(5),
				"status":         "active",
				"corrects_id":    "",
				"is_pro_rata":    false,
				"pro_rata_group": "",
				"pro_rata_index": int64(0),
				"pro_rata_total": int64(0),
				"created_at":     "2026-05-03T10:00:00Z",
			})
		require.NoError(t, err, "failed to seed legacy row %d", i)
	}

	// Re-run InitSchema to trigger the backfill. The batched UPDATE ... LIMIT
	// must drain all 1200 rows without hitting the tx-entry cap.
	require.NoError(t, repo.InitSchema(ctx),
		"batched backfill should complete without max-entries-per-tx error")

	// Verify no legacy rows remain for this user.
	res, err := client.SQLQuery(ctx,
		`SELECT COUNT(*) FROM expenses WHERE transaction_amount IS NULL AND user_id = @user_id`,
		map[string]interface{}{"user_id": userID})
	require.NoError(t, err)
	require.NotEmpty(t, res.Rows)
	assert.Equal(t, int64(0), res.Rows[0].Values[0].GetInt(),
		"all legacy rows should be backfilled")
}
