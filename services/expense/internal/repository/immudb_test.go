package repository

import (
	"context"
	"errors"
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
	// A malformed/short row (fewer than the 25 selected columns) must produce a
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
	assert.Contains(t, err.Error(), "expense row has 3 values, want 25")
}

// legacyRow builds a 25-value row in expenseSelectColumns order with empty
// snapshot fields, simulating a pre-cutover legacy row.
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

// TestRowToExpense_MissingRequiredSnapshotFieldsErrors asserts a row with empty
// snapshot fields is rejected as a data-integrity error. InitSchema backfills
// every legacy row at startup, so a row still missing a required field on read
// must not be synthesized.
func TestRowToExpense_MissingRequiredSnapshotFieldsErrors(t *testing.T) {
	expense, err := rowToExpense(legacyRow())
	require.Error(t, err)
	assert.Nil(t, expense)
	assert.Contains(t, err.Error(), "missing required snapshot fields")
	assert.Contains(t, err.Error(), "exp-1")
}

// TestRowToExpense_IdentitySnapshotRowUsesExplicitFields asserts a row written
// after the cutover returns its explicit snapshot fields unchanged.
func TestRowToExpense_IdentitySnapshotRowUsesExplicitFields(t *testing.T) {
	row := legacyRow()
	// Overwrite the snapshot columns with explicit identity values.
	row.Values[17] = fakeSQLValue{intValue: 1250}                            // transaction_amount
	row.Values[18] = fakeSQLValue{stringValue: "EUR"}                        // transaction_currency
	row.Values[19] = fakeSQLValue{intValue: 1364}                            // reporting_amount
	row.Values[20] = fakeSQLValue{stringValue: "USD"}                        // reporting_currency
	row.Values[21] = fakeSQLValue{stringValue: "1.0912"}                     // exchange_rate
	row.Values[22] = fakeSQLValue{stringValue: model.ExchangeSourceIdentity} // exchange_rate_source
	row.Values[23] = fakeSQLValue{stringValue: "2026-08-14T10:00:00Z"}       // exchange_rate_timestamp

	expense, err := rowToExpense(row)
	require.NoError(t, err)
	assert.Equal(t, int64(1250), expense.TransactionAmount)
	assert.Equal(t, "EUR", expense.TransactionCurrency)
	assert.Equal(t, int64(1364), expense.ReportingAmount)
	assert.Equal(t, "USD", expense.ReportingCurrency)
	assert.Equal(t, "1.0912", expense.ExchangeRate)
	assert.Equal(t, model.ExchangeSourceIdentity, expense.ExchangeRateSource)
	assert.Equal(t, "2026-08-14T10:00:00Z", expense.ExchangeRateTimestamp)
}

// TestRowToExpense_MissingReportingFieldsReturnsIntegrityError asserts a row
// missing a required reporting snapshot field is treated as a data-integrity
// error rather than being silently synthesized or blended with legacy fields.
func TestRowToExpense_MissingReportingFieldsReturnsIntegrityError(t *testing.T) {
	row := legacyRow()
	row.Values[17] = fakeSQLValue{intValue: 1250}     // transaction_amount
	row.Values[18] = fakeSQLValue{stringValue: "EUR"} // transaction_currency
	// reporting_amount, reporting_currency, exchange_rate, exchange_rate_source,
	// and exchange_rate_timestamp are left empty (missing).

	expense, err := rowToExpense(row)
	require.Error(t, err)
	assert.Nil(t, expense)
	assert.Contains(t, err.Error(), "missing required snapshot fields")
	assert.Contains(t, err.Error(), "exp-1")
}

// TestRowToExpense_MissingExchangeRateTimestampReturnsIntegrityError asserts the
// timestamp is a required snapshot field (it is not optional).
func TestRowToExpense_MissingExchangeRateTimestampReturnsIntegrityError(t *testing.T) {
	raw := legacyRow()
	raw.Values[17] = fakeSQLValue{intValue: 2500}
	raw.Values[18] = fakeSQLValue{stringValue: "USD"}
	raw.Values[19] = fakeSQLValue{intValue: 2500}
	raw.Values[20] = fakeSQLValue{stringValue: "USD"}
	raw.Values[21] = fakeSQLValue{stringValue: "1"}
	raw.Values[22] = fakeSQLValue{stringValue: model.ExchangeSourceIdentity}
	raw.Values[23] = fakeSQLValue{stringValue: ""} // exchange_rate_timestamp missing

	_, err := rowToExpense(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required snapshot fields")
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

	assert.Equal(t, 8, client.countQueriesContaining("ALTER TABLE EXPENSES ADD COLUMN"),
		"expected one ALTER per snapshot column")
}

// alterFailingImmudbClient wraps the recording client and fails every ALTER TABLE
// SQLExec with a "column already exists" error, modelling the idempotent
// reconcile path on a table that already has the snapshot columns.
type alterFailingImmudbClient struct {
	*recordingImmudbClient
}

func (c *alterFailingImmudbClient) SQLExec(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	if strings.Contains(strings.ToUpper(sql), "ALTER TABLE") {
		c.record(sql, params)
		return nil, errors.New("column already exists")
	}
	return c.recordingImmudbClient.SQLExec(ctx, sql, params)
}

// TestInitSchema_SwallowsColumnExistsError asserts the reconcile ALTER path is
// idempotent: when the snapshot columns already exist (ALTER returns an
// error), InitSchema still succeeds and issues all 9 ALTER statements.
func TestInitSchema_SwallowsColumnExistsError(t *testing.T) {
	client := &alterFailingImmudbClient{recordingImmudbClient: newRecordingImmudbClient()}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	require.NoError(t, repo.InitSchema(context.Background()))

	assert.Equal(t, 8, client.countQueriesContaining("ALTER TABLE EXPENSES ADD COLUMN"),
		"expected all 8 ALTER statements to be issued even when they fail")
}

// TestInitSchema_BackfillsLegacyRowsAndSkipsRedacted seeds one legacy active
// row (transaction_amount empty) and one redacted row, then asserts InitSchema
// counts exactly the active row and issues a backfill UPDATE whose WHERE clause
// excludes redacted rows (the safety net that keeps anonymized rows untouched).
func TestInitSchema_BackfillsLegacyRowsAndSkipsRedacted(t *testing.T) {
	legacy := buildTestExpense("exp-legacy", "user-1", "2026-05-03T10:00:00Z")
	legacy.TransactionAmount = 0
	redacted := buildTestExpense("exp-redacted", "user-1", "2026-05-04T10:00:00Z")
	redacted.Status = "redacted"
	redacted.TransactionAmount = 0

	client := newRecordingImmudbClient(legacy, redacted)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	require.NoError(t, repo.InitSchema(context.Background()))

	var updateSQL string
	for _, query := range client.Queries() {
		if strings.Contains(strings.ToUpper(query.SQL), "TRANSACTION_AMOUNT = AMOUNT") {
			updateSQL = query.SQL
		}
	}
	require.NotEmpty(t, updateSQL, "expected a backfill UPDATE for the seeded legacy row")
	assert.Contains(t, updateSQL, "status <> 'redacted'")
	assert.Contains(t, updateSQL, "exchange_rate_source = 'migration'")
	assert.Contains(t, updateSQL, "transaction_amount = amount")
	assert.Contains(t, updateSQL, "transaction_currency = currency")
	assert.Contains(t, updateSQL, "reporting_amount = amount")
	assert.Contains(t, updateSQL, "reporting_currency = currency")
	assert.Contains(t, updateSQL, "exchange_rate_timestamp = created_at")
}

// TestInitSchema_SkipsBackfillWhenNoLegacyRows asserts an empty store issues the
// backfill COUNT but no UPDATE, which is what keeps the local in-memory stub
// (empty at startup and UPDATE-averse) from ever reaching the UPDATE.
func TestInitSchema_SkipsBackfillWhenNoLegacyRows(t *testing.T) {
	client := newRecordingImmudbClient()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	require.NoError(t, repo.InitSchema(context.Background()))

	assert.Equal(t, 1, client.countQueriesContaining("TRANSACTION_AMOUNT IS NULL"),
		"expected the backfill COUNT to be issued")
	assert.Equal(t, 0, client.countQueriesContaining("TRANSACTION_AMOUNT = AMOUNT"),
		"no legacy rows means no backfill UPDATE")
}
