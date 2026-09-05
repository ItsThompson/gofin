package repository

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

func TestGetActiveExpenseSuggestionInputs_ReadsActiveRowsForUserAndMapsFields(t *testing.T) {
	client := &fakeImmudbClient{result: &SQLResult{Rows: []SQLRow{{Values: []SQLValue{
		fakeSQLValue{stringValue: "exp-1"},
		fakeSQLValue{stringValue: "Groceries"},
		fakeSQLValue{intValue: 2500},
		fakeSQLValue{stringValue: "USD"},
		fakeSQLValue{stringValue: "essentials"},
		fakeSQLValue{stringValue: "tag-food"},
		fakeSQLValue{stringValue: "2026-05-31T10:00:00Z"},
		fakeSQLValue{stringValue: "2026-05-31"},
		fakeSQLValue{boolValue: true},
		fakeSQLValue{stringValue: "group-1"},
	}}}}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	inputs, err := repo.GetActiveExpenseSuggestionInputs(context.Background(), "user-1")

	require.NoError(t, err)
	assert.Contains(t, client.query, "WHERE user_id = @user_id")
	assert.Contains(t, client.query, "AND status = 'active'")
	assert.Contains(t, client.query, "ORDER BY created_at DESC, id DESC")
	assert.Contains(t, client.query, "LIMIT @limit")
	assert.NotContains(t, strings.ToLower(client.query), "corrected")
	assert.Equal(t, "user-1", client.params["user_id"])
	assert.Equal(t, expenseSuggestionInputLimit, client.params["limit"])
	require.Len(t, inputs, 1)
	assert.Equal(t, "exp-1", inputs[0].ID)
	assert.Equal(t, "Groceries", inputs[0].Name)
	assert.Equal(t, int64(2500), inputs[0].TransactionAmount)
	assert.Equal(t, "USD", inputs[0].TransactionCurrency)
	assert.Equal(t, "essentials", inputs[0].ExpenseType)
	assert.Equal(t, "tag-food", inputs[0].TagID)
	assert.Equal(t, "2026-05-31T10:00:00Z", inputs[0].CreatedAt)
	assert.Equal(t, "2026-05-31", inputs[0].ExpenseDate)
	assert.True(t, inputs[0].IsProRata)
	assert.Equal(t, "group-1", inputs[0].ProRataGroup)
}

// --- GetExpenseByIdempotencyKey tests ---

func TestGetExpenseByIdempotencyKey_ReturnsMatchingRow(t *testing.T) {
	seed := buildTestExpense("exp-1", "user-1", "2026-05-01T10:00:00Z")
	seed.ClientGeneratedIdempotencyKey = "550e8400-e29b-41d4-a716-446655440000"
	client := &fakeImmudbClient{result: expensesToSQLResult([]*model.Expense{seed})}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	expense, err := repo.GetExpenseByIdempotencyKey(context.Background(), "user-1", "550e8400-e29b-41d4-a716-446655440000")

	require.NoError(t, err)
	require.NotNil(t, expense)
	assert.Equal(t, "exp-1", expense.ID)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", expense.ClientGeneratedIdempotencyKey)
	assert.Contains(t, client.query, "WHERE user_id = @user_id AND idempotency_key = @key")
	assert.Equal(t, "user-1", client.params["user_id"])
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", client.params["key"])
}

func TestGetExpenseByIdempotencyKey_ReturnsNilWhenNotFound(t *testing.T) {
	client := &fakeImmudbClient{result: &SQLResult{Rows: nil}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	expense, err := repo.GetExpenseByIdempotencyKey(context.Background(), "user-1", "550e8400-e29b-41d4-a716-446655440000")

	require.NoError(t, err)
	assert.Nil(t, expense)
}

// userFilteredFakeClient wraps fakeImmudbClient but filters its canned result
// rows by the user_id param, so a cross-user query returns no rows, modelling
// the real database's WHERE user_id scope.
type userFilteredFakeClient struct {
	result *SQLResult
	query  string
	params map[string]interface{}
}

func (c *userFilteredFakeClient) SQLExec(context.Context, string, map[string]interface{}) (*SQLResult, error) {
	return &SQLResult{}, nil
}

func (c *userFilteredFakeClient) SQLQuery(_ context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	c.query = sql
	c.params = params
	userID, _ := params["user_id"].(string)
	var filtered []SQLRow
	for _, row := range c.result.Rows {
		// Column 1 is user_id in expenseSelectColumns order.
		if len(row.Values) > 1 && row.Values[1].GetString() == userID {
			filtered = append(filtered, row)
		}
	}
	return &SQLResult{Rows: filtered}, nil
}

func TestGetExpenseByIdempotencyKey_ScopedByUserID(t *testing.T) {
	// Seed a user-1 row; querying as user-2 with the same key must return nil
	// because the lookup is scoped by user_id.
	seed := buildTestExpense("exp-1", "user-1", "2026-05-01T10:00:00Z")
	seed.ClientGeneratedIdempotencyKey = "550e8400-e29b-41d4-a716-446655440000"
	client := &userFilteredFakeClient{result: expensesToSQLResult([]*model.Expense{seed})}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	expense, err := repo.GetExpenseByIdempotencyKey(context.Background(), "user-2", "550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	assert.Nil(t, expense, "another user's expense must not be returned")
	assert.Equal(t, "user-2", client.params["user_id"], "lookup must be scoped to the requesting user")
	assert.Contains(t, client.query, "WHERE user_id = @user_id AND idempotency_key = @key")
}

// --- DeactivateExpense tests ---

func TestDeactivateExpense_IssuesScopedUpdate(t *testing.T) {
	client := newRecordingImmudbClient()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	err := repo.DeactivateExpense(context.Background(), "exp-1", "user-1")
	require.NoError(t, err)

	recorded := client.Queries()
	require.Len(t, recorded, 1)
	upper := strings.ToUpper(recorded[0].SQL)
	assert.Contains(t, upper, "UPDATE EXPENSES SET STATUS = 'CORRECTED'")
	assert.Contains(t, upper, "WHERE ID = @ID AND USER_ID = @USER_ID")
	assert.Equal(t, "exp-1", recorded[0].Params["id"])
	assert.Equal(t, "user-1", recorded[0].Params["user_id"])
}

// --- CreateExpense INSERT includes idempotency_key ---

func TestCreateExpense_IncludesIdempotencyKey(t *testing.T) {
	client := newRecordingImmudbClient()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	expense := buildTestExpense("exp-1", "user-1", "2026-05-01T10:00:00Z")
	expense.ClientGeneratedIdempotencyKey = "550e8400-e29b-41d4-a716-446655440000"

	_, err := repo.CreateExpense(context.Background(), expense)
	require.NoError(t, err)

	inserts := client.countQueriesContaining("INSERT INTO EXPENSES")
	require.Equal(t, 1, inserts)
	recorded := client.Queries()
	assert.Contains(t, strings.ToUpper(recorded[0].SQL), "IDEMPOTENCY_KEY")
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", recorded[0].Params["idempotency_key"])
}
