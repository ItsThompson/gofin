package repository

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// recordedQuery captures a single SQL statement issued through the client, so
// tests can assert query shape (no OFFSET, at most one COUNT, one data query
// per page) without a real database.
type recordedQuery struct {
	SQL    string
	Params map[string]interface{}
}

// recordingImmudbClient is a faithful in-memory simulation of the immudb SQL
// surface used by the expense repository. It records every query and answers
// COUNT(*) and keyset (expanded-OR cursor) data queries over seeded rows, so the
// same client backs the keyset unit tests and the export benchmark. It is safe
// for concurrent use.
type recordingImmudbClient struct {
	mu      sync.Mutex
	rows    []*model.Expense
	queries []recordedQuery
}

func newRecordingImmudbClient(rows ...*model.Expense) *recordingImmudbClient {
	return &recordingImmudbClient{rows: rows}
}

func (c *recordingImmudbClient) record(sql string, params map[string]interface{}) {
	copied := make(map[string]interface{}, len(params))
	for k, v := range params {
		copied[k] = v
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queries = append(c.queries, recordedQuery{SQL: sql, Params: copied})
}

// Queries returns a copy of every query recorded so far.
func (c *recordingImmudbClient) Queries() []recordedQuery {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]recordedQuery, len(c.queries))
	copy(out, c.queries)
	return out
}

// countQueriesContaining returns how many recorded queries contain substr
// (case-insensitive).
func (c *recordingImmudbClient) countQueriesContaining(substr string) int {
	upper := strings.ToUpper(substr)
	n := 0
	for _, q := range c.Queries() {
		if strings.Contains(strings.ToUpper(q.SQL), upper) {
			n++
		}
	}
	return n
}

func (c *recordingImmudbClient) SQLExec(_ context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	c.record(sql, params)
	return &SQLResult{}, nil
}

func (c *recordingImmudbClient) SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.record(sql, params)

	userID, _ := params["user_id"].(string)

	if strings.Contains(strings.ToUpper(sql), "COUNT(*)") {
		var count int64
		for _, row := range c.rows {
			if row.UserID == userID {
				count++
			}
		}
		return &SQLResult{Rows: []SQLRow{{Values: []SQLValue{fakeSQLValue{intValue: count}}}}}, nil
	}

	// Data query: filter to the user, order by (created_at ASC, id ASC).
	matched := make([]*model.Expense, 0, len(c.rows))
	for _, row := range c.rows {
		if row.UserID == userID {
			matched = append(matched, row)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].CreatedAt != matched[j].CreatedAt {
			return matched[i].CreatedAt < matched[j].CreatedAt
		}
		return matched[i].ID < matched[j].ID
	})

	// Keyset seek: expanded-OR (created_at, id) cursor predicate.
	if cursorCreatedAt, ok := params["cursor_created_at"].(string); ok {
		cursorID, _ := params["cursor_id"].(string)
		seeked := matched[:0:0]
		for _, row := range matched {
			if row.CreatedAt > cursorCreatedAt ||
				(row.CreatedAt == cursorCreatedAt && row.ID > cursorID) {
				seeked = append(seeked, row)
			}
		}
		matched = seeked
	}

	page := matched

	// LIMIT.
	if limit, ok := params["limit"]; ok {
		if l := int(toInt64(limit)); l >= 0 && l < len(page) {
			page = page[:l]
		}
	}

	return expensesToSQLResult(page), nil
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case int:
		return int64(val)
	default:
		return 0
	}
}

// expensesToSQLResult renders expenses as SQL rows in the column order the
// repository's SELECT clauses (expenseSelectColumns) expect.
func expensesToSQLResult(expenses []*model.Expense) *SQLResult {
	rows := make([]SQLRow, 0, len(expenses))
	for _, e := range expenses {
		rows = append(rows, SQLRow{Values: []SQLValue{
			fakeSQLValue{stringValue: e.ID},
			fakeSQLValue{stringValue: e.UserID},
			fakeSQLValue{stringValue: e.Name},
			fakeSQLValue{intValue: e.Amount},
			fakeSQLValue{stringValue: e.Currency},
			fakeSQLValue{stringValue: e.ExpenseType},
			fakeSQLValue{stringValue: e.TagID},
			fakeSQLValue{stringValue: e.ExpenseDate},
			fakeSQLValue{intValue: int64(e.PeriodYear)},
			fakeSQLValue{intValue: int64(e.PeriodMonth)},
			fakeSQLValue{stringValue: e.Status},
			fakeSQLValue{stringValue: e.CorrectsID},
			fakeSQLValue{boolValue: e.IsProRata},
			fakeSQLValue{stringValue: e.ProRataGroup},
			fakeSQLValue{intValue: int64(e.ProRataIndex)},
			fakeSQLValue{intValue: int64(e.ProRataTotal)},
			fakeSQLValue{stringValue: e.CreatedAt},
			fakeSQLValue{intValue: int64(e.MoneySnapshotVersion)},
			fakeSQLValue{intValue: e.TransactionAmount},
			fakeSQLValue{stringValue: e.TransactionCurrency},
			fakeSQLValue{intValue: e.ReportingAmount},
			fakeSQLValue{stringValue: e.ReportingCurrency},
			fakeSQLValue{stringValue: e.ExchangeRate},
			fakeSQLValue{stringValue: e.ExchangeRateSource},
			fakeSQLValue{stringValue: e.ExchangeRateTimestamp},
			fakeSQLValue{stringValue: e.ExchangeRateExpiresAt},
		}})
	}
	return &SQLResult{Rows: rows}
}

// buildTestExpense constructs an expense row for repository tests with the given
// id, userID, and createdAt and default values for the remaining fields.
func buildTestExpense(id, userID, createdAt string) *model.Expense {
	return &model.Expense{
		ID:          id,
		UserID:      userID,
		Name:        "Expense " + id,
		Amount:      1000,
		Currency:    "USD",
		ExpenseType: "essentials",
		TagID:       "tag-1",
		ExpenseDate: "2026-05-01",
		PeriodYear:  2026,
		PeriodMonth: 5,
		Status:      "active",
		CreatedAt:   createdAt,
	}
}
