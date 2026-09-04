package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// expenseSelectColumns is the shared SELECT column list for expense queries.
const expenseSelectColumns = `id, user_id, name, expense_type, tag_id,
		expense_date, period_year, period_month, status, corrects_id,
		is_pro_rata, pro_rata_group, pro_rata_index, pro_rata_total, created_at,
		transaction_amount, transaction_currency,
		reporting_amount, reporting_currency, exchange_rate, exchange_rate_source,
		exchange_rate_timestamp, exchange_rate_expires_at, idempotency_key`

// expenseColumnCount is the number of columns rowToExpense expects in a result
// row. It must match the expenseSelectColumns list and the ExpenseData schema.
const expenseColumnCount = 24

const expenseSuggestionInputLimit int32 = 1000

// mapRow wraps rowToExpense with telemetry. When rowToExpense returns a
// SnapshotIntegrityError (row missing required fields), it logs the
// expense_snapshot_integrity_error event before returning the error, so the
// data-integrity issue is recorded even though the read fails.
func (r *ImmudbExpenseRepository) mapRow(row SQLRow) (*model.Expense, error) {
	expense, err := rowToExpense(row)
	if err == nil {
		return expense, nil
	}
	var integrityErr *SnapshotIntegrityError
	if errors.As(err, &integrityErr) {
		r.logger.Warn("expense snapshot integrity error",
			slog.String("event", "expense_snapshot_integrity_error"),
			slog.String("expense_id", integrityErr.ExpenseID),
			slog.Any("missing_fields", integrityErr.MissingFields),
		)
	}
	return nil, err
}

// rowToExpense maps a result row to an Expense. The column order must match the
// SELECT clause in queries. It returns an error on a short/malformed row rather
// than panicking on an out-of-range index.
//
// Every row must carry the required snapshot fields. Rows written after the
// multi-currency cutover always populate them, so a row still missing a
// required field on read is a data-integrity error.
func rowToExpense(row SQLRow) (*model.Expense, error) {
	values := row.Values
	if len(values) < expenseColumnCount {
		return nil, fmt.Errorf("expense row has %d values, want %d", len(values), expenseColumnCount)
	}
	exp := &model.Expense{
		ID:                            values[0].GetString(),
		UserID:                        values[1].GetString(),
		Name:                          values[2].GetString(),
		ExpenseType:                   values[3].GetString(),
		TagID:                         values[4].GetString(),
		ExpenseDate:                   values[5].GetString(),
		PeriodYear:                    int32(values[6].GetInt()),
		PeriodMonth:                   int32(values[7].GetInt()),
		Status:                        values[8].GetString(),
		CorrectsID:                    values[9].GetString(),
		IsProRata:                     values[10].GetBool(),
		ProRataGroup:                  values[11].GetString(),
		ProRataIndex:                  int32(values[12].GetInt()),
		ProRataTotal:                  int32(values[13].GetInt()),
		CreatedAt:                     values[14].GetString(),
		TransactionAmount:             values[15].GetInt(),
		TransactionCurrency:           values[16].GetString(),
		ReportingAmount:               values[17].GetInt(),
		ReportingCurrency:             values[18].GetString(),
		ExchangeRate:                  values[19].GetString(),
		ExchangeRateSource:            values[20].GetString(),
		ExchangeRateTimestamp:         values[21].GetString(),
		ExchangeRateExpiresAt:         values[22].GetString(),
		ClientGeneratedIdempotencyKey: values[23].GetString(),
	}

	missing := make([]string, 0, 7)
	if exp.TransactionAmount == 0 {
		missing = append(missing, "transaction_amount")
	}
	if exp.TransactionCurrency == "" {
		missing = append(missing, "transaction_currency")
	}
	if exp.ReportingAmount == 0 {
		missing = append(missing, "reporting_amount")
	}
	if exp.ReportingCurrency == "" {
		missing = append(missing, "reporting_currency")
	}
	if exp.ExchangeRate == "" {
		missing = append(missing, "exchange_rate")
	}
	if exp.ExchangeRateSource == "" {
		missing = append(missing, "exchange_rate_source")
	}
	if exp.ExchangeRateTimestamp == "" {
		missing = append(missing, "exchange_rate_timestamp")
	}
	if len(missing) > 0 {
		return nil, &SnapshotIntegrityError{ExpenseID: exp.ID, MissingFields: missing}
	}

	return exp, nil
}

// Column order must match GetActiveExpenseSuggestionInputs SELECT clause.
func rowToExpenseSuggestionInput(row SQLRow) *model.ExpenseSuggestionInput {
	values := row.Values
	return &model.ExpenseSuggestionInput{
		ID:                  values[0].GetString(),
		Name:                values[1].GetString(),
		TransactionAmount:   values[2].GetInt(),
		TransactionCurrency: values[3].GetString(),
		ExpenseType:         values[4].GetString(),
		TagID:               values[5].GetString(),
		CreatedAt:           values[6].GetString(),
		ExpenseDate:         values[7].GetString(),
		IsProRata:           values[8].GetBool(),
		ProRataGroup:        values[9].GetString(),
	}
}

// GetActiveExpenseSuggestionInputs returns recent active expense rows for suggestion ranking.
func (r *ImmudbExpenseRepository) GetActiveExpenseSuggestionInputs(ctx context.Context, userID string) ([]*model.ExpenseSuggestionInput, error) {
	query := `SELECT id, name, transaction_amount, transaction_currency,
		expense_type, tag_id, created_at, expense_date, is_pro_rata, pro_rata_group
		FROM expenses
		WHERE user_id = @user_id
		AND status = 'active'
		ORDER BY created_at DESC, id DESC
		LIMIT @limit;`

	result, err := r.client.SQLQuery(ctx, query, map[string]interface{}{
		"user_id": userID,
		"limit":   expenseSuggestionInputLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("querying active expense suggestion inputs: %w", err)
	}

	inputs := make([]*model.ExpenseSuggestionInput, 0, len(result.Rows))
	for _, row := range result.Rows {
		inputs = append(inputs, rowToExpenseSuggestionInput(row))
	}

	return inputs, nil
}
