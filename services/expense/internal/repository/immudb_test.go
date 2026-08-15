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

func TestGetExpenseByID_ShortRowReturnsErrorNotPanic(t *testing.T) {
	// A malformed/short row (fewer than the 17 selected columns) must produce a
	// wrapped error rather than an index-out-of-range panic.
	client := &fakeImmudbClient{result: &SQLResult{Rows: []SQLRow{{Values: []SQLValue{
		fakeSQLValue{stringValue: "exp-1"},
		fakeSQLValue{stringValue: "user-1"},
		fakeSQLValue{stringValue: "Groceries"},
	}}}}}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	expense, err := repo.GetExpenseByID(context.Background(), "exp-1", "user-1")

	require.Error(t, err)
	assert.Nil(t, expense)
	assert.Contains(t, err.Error(), "expense row has 3 values, want 26")
}

// legacyRow builds a 26-value row in expenseSelectColumns order with no
// money snapshot (version 0/empty), simulating a pre-cutover legacy row.
func legacyRow() SQLRow {
	return SQLRow{Values: []SQLValue{
		fakeSQLValue{stringValue: "exp-1"},
		fakeSQLValue{stringValue: "user-1"},
		fakeSQLValue{stringValue: "Groceries"},
		fakeSQLValue{intValue: 2500},
		fakeSQLValue{stringValue: "USD"},
		fakeSQLValue{stringValue: "essentials"},
		fakeSQLValue{stringValue: "tag-food"},
		fakeSQLValue{stringValue: "2026-05-03"},
		fakeSQLValue{intValue: 2026},
		fakeSQLValue{intValue: 5},
		fakeSQLValue{stringValue: "active"},
		fakeSQLValue{stringValue: ""},
		fakeSQLValue{boolValue: false},
		fakeSQLValue{stringValue: ""},
		fakeSQLValue{intValue: 0},
		fakeSQLValue{intValue: 0},
		fakeSQLValue{stringValue: "2026-05-03T10:00:00Z"},
		fakeSQLValue{intValue: 0},     // money_snapshot_version
		fakeSQLValue{intValue: 0},     // transaction_amount
		fakeSQLValue{stringValue: ""}, // transaction_currency
		fakeSQLValue{intValue: 0},     // reporting_amount
		fakeSQLValue{stringValue: ""}, // reporting_currency
		fakeSQLValue{stringValue: ""}, // exchange_rate
		fakeSQLValue{stringValue: ""}, // exchange_rate_source
		fakeSQLValue{stringValue: ""}, // exchange_rate_timestamp
		fakeSQLValue{stringValue: ""}, // exchange_rate_expires_at
	}}
}

// TestRowToExpense_LegacyRowSynthesizesMigrationSnapshot asserts a row with no
// money snapshot (version absent/null) reads without panics and is synthesized
// into a migration snapshot: legacy amount is both transaction and reporting
// money in the legacy currency, with rate "1" and source "migration".
func TestRowToExpense_LegacyRowSynthesizesMigrationSnapshot(t *testing.T) {
	expense, err := rowToExpense(legacyRow())
	require.NoError(t, err)
	assert.Equal(t, int32(1), expense.MoneySnapshotVersion) // synthesized present
	assert.Equal(t, int64(2500), expense.TransactionAmount)
	assert.Equal(t, "USD", expense.TransactionCurrency)
	assert.Equal(t, int64(2500), expense.ReportingAmount)
	assert.Equal(t, "USD", expense.ReportingCurrency)
	assert.Equal(t, "1", expense.ExchangeRate)
	assert.Equal(t, model.ExchangeSourceMigration, expense.ExchangeRateSource)
	assert.Equal(t, "2026-05-03T10:00:00Z", expense.ExchangeRateTimestamp)
	assert.Empty(t, expense.ExchangeRateExpiresAt)
}

// TestRowToExpense_IdentitySnapshotRowUsesExplicitFields asserts a row written
// after the cutover (version 1) returns its explicit snapshot fields unchanged.
func TestRowToExpense_IdentitySnapshotRowUsesExplicitFields(t *testing.T) {
	row := legacyRow()
	// Overwrite the snapshot columns with explicit identity values.
	row.Values[17] = fakeSQLValue{intValue: 1}                               // money_snapshot_version
	row.Values[18] = fakeSQLValue{intValue: 1250}                            // transaction_amount
	row.Values[19] = fakeSQLValue{stringValue: "EUR"}                        // transaction_currency
	row.Values[20] = fakeSQLValue{intValue: 1364}                            // reporting_amount
	row.Values[21] = fakeSQLValue{stringValue: "USD"}                        // reporting_currency
	row.Values[22] = fakeSQLValue{stringValue: "1.0912"}                     // exchange_rate
	row.Values[23] = fakeSQLValue{stringValue: model.ExchangeSourceIdentity} // exchange_rate_source
	row.Values[24] = fakeSQLValue{stringValue: "2026-08-14T10:00:00Z"}       // exchange_rate_timestamp

	expense, err := rowToExpense(row)
	require.NoError(t, err)
	assert.Equal(t, int32(1), expense.MoneySnapshotVersion)
	assert.Equal(t, int64(1250), expense.TransactionAmount)
	assert.Equal(t, "EUR", expense.TransactionCurrency)
	assert.Equal(t, int64(1364), expense.ReportingAmount)
	assert.Equal(t, "USD", expense.ReportingCurrency)
	assert.Equal(t, "1.0912", expense.ExchangeRate)
	assert.Equal(t, model.ExchangeSourceIdentity, expense.ExchangeRateSource)
	assert.Equal(t, "2026-08-14T10:00:00Z", expense.ExchangeRateTimestamp)
}

// TestInitSchema_ReconcilesSnapshotColumns asserts InitSchema issues the ALTER
// TABLE ADD COLUMN statements for the money snapshot columns and does not fail
// when the columns already exist (the recording client returns errors that the
// reconcile path swallows).
func TestInitSchema_ReconcilesSnapshotColumns(t *testing.T) {
	client := newRecordingImmudbClient()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	require.NoError(t, repo.InitSchema(context.Background()))

	assert.Equal(t, 9, client.countQueriesContaining("ALTER TABLE EXPENSES ADD COLUMN"),
		"expected one ALTER per snapshot column")
}
