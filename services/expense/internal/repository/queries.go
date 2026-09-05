package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// CreateExpense inserts a new expense entry into the immudb ledger.
func (r *ImmudbExpenseRepository) CreateExpense(ctx context.Context, expense *model.Expense) (*model.Expense, error) {
	query := `INSERT INTO expenses (
		id, user_id, name, expense_type, tag_id,
		expense_date, period_year, period_month, status, corrects_id,
		is_pro_rata, pro_rata_group, pro_rata_index, pro_rata_total, created_at,
		transaction_amount, transaction_currency,
		reporting_amount, reporting_currency, exchange_rate, exchange_rate_source,
		exchange_rate_timestamp, exchange_rate_expires_at, idempotency_key
	) VALUES (
		@id, @user_id, @name, @expense_type, @tag_id,
		@expense_date, @period_year, @period_month, @status, @corrects_id,
		@is_pro_rata, @pro_rata_group, @pro_rata_index, @pro_rata_total, @created_at,
		@transaction_amount, @transaction_currency,
		@reporting_amount, @reporting_currency, @exchange_rate, @exchange_rate_source,
		@exchange_rate_timestamp, @exchange_rate_expires_at, @idempotency_key
	);`

	params := map[string]interface{}{
		"id":                       expense.ID,
		"user_id":                  expense.UserID,
		"name":                     expense.Name,
		"expense_type":             expense.ExpenseType,
		"tag_id":                   expense.TagID,
		"expense_date":             expense.ExpenseDateIso,
		"period_year":              expense.PeriodYear,
		"period_month":             expense.PeriodMonth,
		"status":                   expense.Status,
		"corrects_id":              expense.CorrectsID,
		"is_pro_rata":              expense.IsProRata,
		"pro_rata_group":           expense.ProRataGroup,
		"pro_rata_index":           expense.ProRataIndex,
		"pro_rata_total":           expense.ProRataTotal,
		"created_at":               expense.CreatedAt,
		"transaction_amount":       expense.OriginalTransactionAmountInMinorUnits,
		"transaction_currency":     expense.TransactionCurrencyCode,
		"reporting_amount":         expense.ReportingAmountInMinorUnits,
		"reporting_currency":       expense.ReportingCurrencyCode,
		"exchange_rate":            expense.SourceToTargetExchangeRate,
		"exchange_rate_source":     expense.ExchangeRateSource,
		"exchange_rate_timestamp":  expense.ExchangeRateTimestamp,
		"exchange_rate_expires_at": expense.ExchangeRateCacheExpiresAt,
		"idempotency_key":          expense.ClientGeneratedIdempotencyKey,
	}

	_, err := r.client.SQLExec(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("inserting expense: %w", err)
	}

	return expense, nil
}

// GetActiveExpensesForPeriod returns materialized (active-only) expenses for the given
// user and period, with pagination. Also returns the total count.
func (r *ImmudbExpenseRepository) GetActiveExpensesForPeriod(ctx context.Context, userID string, year, month, page, pageSize int32) ([]*model.Expense, int64, error) {
	// Count query for pagination
	countQuery := `SELECT COUNT(*) FROM expenses
		WHERE user_id = @user_id
		AND period_year = @year
		AND period_month = @month
		AND status = 'active';`

	countParams := map[string]interface{}{
		"user_id": userID,
		"year":    year,
		"month":   month,
	}

	countResult, err := r.client.SQLQuery(ctx, countQuery, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("counting expenses: %w", err)
	}

	var total int64
	if len(countResult.Rows) > 0 && len(countResult.Rows[0].Values) > 0 {
		total = countResult.Rows[0].Values[0].GetInt()
	}

	// Data query with pagination, ordered by expense_date DESC, created_at DESC
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(`SELECT %s
		FROM expenses
		WHERE user_id = @user_id
		AND period_year = @year
		AND period_month = @month
		AND status = 'active'
		ORDER BY expense_date DESC, created_at DESC
		LIMIT @limit OFFSET @offset;`, expenseSelectColumns)

	dataParams := map[string]interface{}{
		"user_id": userID,
		"year":    year,
		"month":   month,
		"limit":   pageSize,
		"offset":  offset,
	}

	result, err := r.client.SQLQuery(ctx, dataQuery, dataParams)
	if err != nil {
		return nil, 0, fmt.Errorf("querying expenses: %w", err)
	}

	expenses := make([]*model.Expense, 0, len(result.Rows))
	for _, row := range result.Rows {
		expense, convErr := r.mapRow(row)
		if convErr != nil {
			return nil, 0, fmt.Errorf("mapping expense row: %w", convErr)
		}
		expenses = append(expenses, expense)
	}

	return expenses, total, nil
}

// GetExpenseByID returns a single expense by ID, scoped to the given user.
// Returns nil if not found or if the expense belongs to a different user.
func (r *ImmudbExpenseRepository) GetExpenseByID(ctx context.Context, id string, userID string) (*model.Expense, error) {
	query := fmt.Sprintf(`SELECT %s
		FROM expenses
		WHERE id = @id AND user_id = @user_id;`, expenseSelectColumns)

	result, err := r.client.SQLQuery(ctx, query, map[string]interface{}{"id": id, "user_id": userID})
	if err != nil {
		return nil, fmt.Errorf("querying expense by ID: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, nil
	}

	expense, err := r.mapRow(result.Rows[0])
	if err != nil {
		return nil, fmt.Errorf("mapping expense row: %w", err)
	}
	return expense, nil
}

// GetExpenseByIdempotencyKey returns the expense for the given user that was
// created with the supplied idempotency key, or nil if no such expense exists.
func (r *ImmudbExpenseRepository) GetExpenseByIdempotencyKey(ctx context.Context, userID string, key string) (*model.Expense, error) {
	query := fmt.Sprintf(`SELECT %s
		FROM expenses
		WHERE user_id = @user_id AND idempotency_key = @key;`, expenseSelectColumns)

	result, err := r.client.SQLQuery(ctx, query, map[string]interface{}{
		"user_id": userID,
		"key":     key,
	})
	if err != nil {
		return nil, fmt.Errorf("querying expense by idempotency key: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, nil
	}

	expense, err := r.mapRow(result.Rows[0])
	if err != nil {
		return nil, fmt.Errorf("mapping expense row: %w", err)
	}
	return expense, nil
}

// DeactivateExpense soft-deletes an active expense by flipping its status to
// "corrected" with no replacement row. Scoped to the user. immudb's SQLExec
// does not expose a row count; the service layer's pre-check (fetch -> 404 if
// nil -> reject if not active) guarantees the row exists and is active before
// this call, so a successful exec means the UPDATE ran. Returns 1 on success.
func (r *ImmudbExpenseRepository) DeactivateExpense(ctx context.Context, id string, userID string) (int64, error) {
	query := `UPDATE expenses SET status = 'corrected' WHERE id = @id AND user_id = @user_id;`
	_, err := r.client.SQLExec(ctx, query, map[string]interface{}{
		"id":      id,
		"user_id": userID,
	})
	if err != nil {
		return 0, fmt.Errorf("deactivating expense: %w", err)
	}
	return 1, nil
}

// CountExpensesByTag returns the count of active expenses referencing the given tag
// for a specific user. Used by the finance service to check tag usage before deletion.
func (r *ImmudbExpenseRepository) CountExpensesByTag(ctx context.Context, userID string, tagID string) (int64, error) {
	query := `SELECT COUNT(*) FROM expenses
		WHERE user_id = @user_id
		AND tag_id = @tag_id
		AND status = 'active';`

	result, err := r.client.SQLQuery(ctx, query, map[string]interface{}{
		"user_id": userID,
		"tag_id":  tagID,
	})
	if err != nil {
		return 0, fmt.Errorf("counting expenses by tag: %w", err)
	}

	if len(result.Rows) > 0 && len(result.Rows[0].Values) > 0 {
		return result.Rows[0].Values[0].GetInt(), nil
	}

	return 0, nil
}

// GetExpensesInProRataGroup returns all expenses in a pro-rata group for a user,
// ordered by installment index.
func (r *ImmudbExpenseRepository) GetExpensesInProRataGroup(ctx context.Context, groupID string, userID string) ([]*model.Expense, error) {
	query := fmt.Sprintf(`SELECT %s FROM expenses
		WHERE pro_rata_group = @group_id AND user_id = @user_id
		ORDER BY pro_rata_index;`, expenseSelectColumns)

	result, err := r.client.SQLQuery(ctx, query, map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
	})
	if err != nil {
		return nil, fmt.Errorf("querying pro-rata group: %w", err)
	}

	expenses := make([]*model.Expense, 0, len(result.Rows))
	for _, row := range result.Rows {
		expense, convErr := r.mapRow(row)
		if convErr != nil {
			return nil, fmt.Errorf("mapping expense row: %w", convErr)
		}
		expenses = append(expenses, expense)
	}

	return expenses, nil
}

// AnonymizeAllUserExpenses redacts PII fields on all expense rows for a user.
// The UPDATE overwrites the current head in immudb's append-only model.
// Idempotent: re-calling for already-redacted data produces the same result.
// Returns nil when zero rows match (user has no expenses).
func (r *ImmudbExpenseRepository) AnonymizeAllUserExpenses(ctx context.Context, userID string) error {
	query := `UPDATE expenses SET
		user_id = 'DELETED',
		name = 'REDACTED',
		expense_type = '',
		tag_id = '',
		expense_date = '',
		status = 'redacted',
		transaction_amount = 0,
		transaction_currency = '',
		reporting_amount = 0,
		reporting_currency = '',
		exchange_rate = '',
		exchange_rate_source = '',
		exchange_rate_timestamp = '',
		exchange_rate_expires_at = ''
		WHERE user_id = @user_id;`

	_, err := r.client.SQLExec(ctx, query, map[string]interface{}{
		"user_id": userID,
	})
	if err != nil {
		return fmt.Errorf("anonymizing user expenses: %w", err)
	}

	r.logger.Info("user expenses anonymized",
		slog.String("user_id", userID),
	)

	return nil
}
