package repository

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSQLValue struct {
	stringValue string
	intValue    int64
	boolValue   bool
}

func (value fakeSQLValue) GetString() string { return value.stringValue }
func (value fakeSQLValue) GetInt() int64     { return value.intValue }
func (value fakeSQLValue) GetBool() bool     { return value.boolValue }

type fakeImmudbClient struct {
	query  string
	params map[string]interface{}
	result *SQLResult
}

func (client *fakeImmudbClient) SQLExec(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	return &SQLResult{}, nil
}

func (client *fakeImmudbClient) SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	client.query = sql
	client.params = params
	return client.result, nil
}

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
	assert.Equal(t, int64(2500), inputs[0].Amount)
	assert.Equal(t, "USD", inputs[0].Currency)
	assert.Equal(t, "essentials", inputs[0].ExpenseType)
	assert.Equal(t, "tag-food", inputs[0].TagID)
	assert.Equal(t, "2026-05-31T10:00:00Z", inputs[0].CreatedAt)
	assert.Equal(t, "2026-05-31", inputs[0].ExpenseDate)
	assert.True(t, inputs[0].IsProRata)
	assert.Equal(t, "group-1", inputs[0].ProRataGroup)
}
