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

// --- GetExpenseByIdempotencyKey tests ---

func TestGetExpenseByIdempotencyKey_ReturnsMatchingRow(t *testing.T) {
	seed := buildTestExpense("exp-1", "user-1", "2026-05-01T10:00:00Z")
	seed.IdempotencyKey = "550e8400-e29b-41d4-a716-446655440000"
	client := &fakeImmudbClient{result: expensesToSQLResult([]*model.Expense{seed})}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	expense, err := repo.GetExpenseByIdempotencyKey(context.Background(), "user-1", "550e8400-e29b-41d4-a716-446655440000")

	require.NoError(t, err)
	require.NotNil(t, expense)
	assert.Equal(t, "exp-1", expense.ID)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", expense.IdempotencyKey)
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

func TestGetExpenseByIdempotencyKey_ScopedByUserID(t *testing.T) {
	// The query must carry the user_id scope so another user's row with the same
	// key is not returned. Assert the WHERE clause and params enforce the scope.
	seed := buildTestExpense("exp-1", "user-1", "2026-05-01T10:00:00Z")
	client := &fakeImmudbClient{result: expensesToSQLResult([]*model.Expense{seed})}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	_, err := repo.GetExpenseByIdempotencyKey(context.Background(), "user-2", "550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)

	assert.Equal(t, "user-2", client.params["user_id"], "lookup must be scoped to the requesting user")
}

// --- DeactivateExpense tests ---

func TestDeactivateExpense_IssuesScopedUpdate(t *testing.T) {
	client := newRecordingImmudbClient()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	count, err := repo.DeactivateExpense(context.Background(), "exp-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

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
	expense.IdempotencyKey = "550e8400-e29b-41d4-a716-446655440000"

	_, err := repo.CreateExpense(context.Background(), expense)
	require.NoError(t, err)

	inserts := client.countQueriesContaining("INSERT INTO EXPENSES")
	require.Equal(t, 1, inserts)
	recorded := client.Queries()
	assert.Contains(t, strings.ToUpper(recorded[0].SQL), "IDEMPOTENCY_KEY")
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", recorded[0].Params["idempotency_key"])
}

// --- InitSchema reconciles idempotency_key column ---

func TestInitSchema_ReconcilesIdempotencyKeyColumn(t *testing.T) {
	client := newRecordingImmudbClient()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	require.NoError(t, repo.InitSchema(context.Background()))

	// The idempotency_key ALTER is issued as part of the reconcile loop.
	assert.Equal(t, 1, client.countQueriesContaining("ALTER TABLE EXPENSES ADD COLUMN IDEMPOTENCY_KEY"),
		"expected an ALTER for the idempotency_key column")
	// The idempotency-key lookup index is created.
	assert.Equal(t, 1, client.countQueriesContaining("IDX_EXPENSES_USER_IDEM"),
		"expected the idempotency-key lookup index to be created")
}