package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// ImmudbExpenseRepository implements ExpenseRepository using immudb's SQL interface.
type ImmudbExpenseRepository struct {
	client ImmudbClient
	logger *slog.Logger
}

// NewImmudbExpenseRepository creates a new ImmudbExpenseRepository.
func NewImmudbExpenseRepository(client ImmudbClient, logger *slog.Logger) *ImmudbExpenseRepository {
	return &ImmudbExpenseRepository{
		client: client,
		logger: logger,
	}
}

// InitSchema creates the expenses table and indexes if they don't exist, and
// reconciles additive money-snapshot columns onto pre-existing tables.
func (r *ImmudbExpenseRepository) InitSchema(ctx context.Context) error {
	createTable := `CREATE TABLE IF NOT EXISTS expenses (
		id              VARCHAR(36)  NOT NULL,
		user_id         VARCHAR(36)  NOT NULL,
		name            VARCHAR(255) NOT NULL,
		amount          INTEGER      NOT NULL,
		currency        VARCHAR(3)   NOT NULL,
		expense_type    VARCHAR(20)  NOT NULL,
		tag_id          VARCHAR(36)  NOT NULL,
		expense_date    VARCHAR(10)  NOT NULL,
		period_year     INTEGER      NOT NULL,
		period_month    INTEGER      NOT NULL,
		status          VARCHAR(20)  NOT NULL,
		corrects_id     VARCHAR(36),
		is_pro_rata     BOOLEAN      NOT NULL,
		pro_rata_group  VARCHAR(36),
		pro_rata_index  INTEGER,
		pro_rata_total  INTEGER,
		created_at      VARCHAR(30)  NOT NULL,
		transaction_amount      INTEGER,
		transaction_currency    VARCHAR(3),
		reporting_amount        INTEGER,
		reporting_currency      VARCHAR(3),
		exchange_rate           VARCHAR(40),
		exchange_rate_source    VARCHAR(40),
		exchange_rate_timestamp VARCHAR(30),
		exchange_rate_expires_at VARCHAR(30),
		PRIMARY KEY (id)
	);`

	_, err := r.client.SQLExec(ctx, createTable, nil)
	if err != nil {
		return fmt.Errorf("creating expenses table: %w", err)
	}

	// Reconcile additive money-snapshot columns onto tables created before the
	// multi-currency cutover. CREATE TABLE IF NOT EXISTS is a no-op for an existing
	// table, so the new nullable columns would be missing on pre-cutover tables.
	// ALTER TABLE ADD COLUMN adds them; a column that already exists returns an
	// error, which we swallow so reconciliation is idempotent.
	reconcileColumns := []string{
		`ALTER TABLE expenses ADD COLUMN transaction_amount INTEGER;`,
		`ALTER TABLE expenses ADD COLUMN transaction_currency VARCHAR(3);`,
		`ALTER TABLE expenses ADD COLUMN reporting_amount INTEGER;`,
		`ALTER TABLE expenses ADD COLUMN reporting_currency VARCHAR(3);`,
		`ALTER TABLE expenses ADD COLUMN exchange_rate VARCHAR(40);`,
		`ALTER TABLE expenses ADD COLUMN exchange_rate_source VARCHAR(40);`,
		`ALTER TABLE expenses ADD COLUMN exchange_rate_timestamp VARCHAR(30);`,
		`ALTER TABLE expenses ADD COLUMN exchange_rate_expires_at VARCHAR(30);`,
	}
	for _, stmt := range reconcileColumns {
		if _, addErr := r.client.SQLExec(ctx, stmt, nil); addErr != nil {
			// Expected when the column already exists (idempotent reconcile).
			r.logger.Debug("snapshot column reconcile skipped (may already exist)",
				slog.String("statement", stmt),
				slog.String("error", addErr.Error()),
			)
		}
	}

	// Run the one-time legacy money-snapshot migration after the columns exist.
	if err := r.backfillMoneySnapshots(ctx); err != nil {
		return err
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_expenses_user_period ON expenses (user_id, period_year, period_month, status);`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_corrects ON expenses (corrects_id);`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_prorata_group ON expenses (pro_rata_group);`,
		// Covers the keyset export seek: the (created_at, id) tiebreaker column is
		// included so the index matches the full ORDER BY created_at ASC, id ASC
		// tuple and rows tied on created_at seek instead of scanning+sorting.
		`CREATE INDEX IF NOT EXISTS idx_expenses_user_created_id ON expenses (user_id, created_at, id);`,
	}

	for _, idx := range indexes {
		_, err := r.client.SQLExec(ctx, idx, nil)
		if err != nil {
			// Index creation may fail if it already exists; log and continue.
			r.logger.Warn("index creation skipped (may already exist)",
				slog.String("error", err.Error()),
			)
		}
	}

	r.logger.Info("immudb schema initialized")
	return nil
}

// backfillMoneySnapshots backfills rows missing a transaction amount with an
// identity snapshot in their legacy currency. It is idempotent: count first,
// then run the UPDATE only when such rows exist, which also keeps the local
// in-memory stub (empty at startup and UPDATE-averse) from ever reaching the
// UPDATE.
//
// TODO: delete this method once the mc/03 migration has shipped and prod has
// booted with the new code. It is a one-time data migration, not startup logic.
func (r *ImmudbExpenseRepository) backfillMoneySnapshots(ctx context.Context) error {
	backfillCountQuery := `SELECT COUNT(*) FROM expenses
		WHERE transaction_amount IS NULL
		AND status <> 'redacted';`
	countResult, err := r.client.SQLQuery(ctx, backfillCountQuery, nil)
	if err != nil {
		return fmt.Errorf("counting legacy money snapshot rows: %w", err)
	}
	var legacyRows int64
	if len(countResult.Rows) > 0 && len(countResult.Rows[0].Values) > 0 {
		legacyRows = countResult.Rows[0].Values[0].GetInt()
	}
	r.logger.Info("money_snapshot_backfill check",
		slog.String("event", "money_snapshot_backfill"),
		slog.Int64("legacy_rows", legacyRows),
	)
	if legacyRows == 0 {
		return nil
	}

	backfillUpdate := fmt.Sprintf(`UPDATE expenses SET
		transaction_amount = amount,
		transaction_currency = currency,
		reporting_amount = amount,
		reporting_currency = currency,
		exchange_rate = '1',
		exchange_rate_source = '%s',
		exchange_rate_timestamp = created_at
		WHERE transaction_amount IS NULL
		AND status <> 'redacted';`, model.ExchangeSourceMigration)
	if _, err := r.client.SQLExec(ctx, backfillUpdate, nil); err != nil {
		return fmt.Errorf("backfilling legacy money snapshots: %w", err)
	}
	return nil
}

// CreateExpense inserts a new expense entry into the immudb ledger.
func (r *ImmudbExpenseRepository) CreateExpense(ctx context.Context, expense *model.Expense) (*model.Expense, error) {
	query := `INSERT INTO expenses (
		id, user_id, name, amount, currency, expense_type, tag_id,
		expense_date, period_year, period_month, status, corrects_id,
		is_pro_rata, pro_rata_group, pro_rata_index, pro_rata_total, created_at,
		transaction_amount, transaction_currency,
		reporting_amount, reporting_currency, exchange_rate, exchange_rate_source,
		exchange_rate_timestamp, exchange_rate_expires_at
	) VALUES (
		@id, @user_id, @name, @amount, @currency, @expense_type, @tag_id,
		@expense_date, @period_year, @period_month, @status, @corrects_id,
		@is_pro_rata, @pro_rata_group, @pro_rata_index, @pro_rata_total, @created_at,
		@transaction_amount, @transaction_currency,
		@reporting_amount, @reporting_currency, @exchange_rate, @exchange_rate_source,
		@exchange_rate_timestamp, @exchange_rate_expires_at
	);`

	params := map[string]interface{}{
		"id":                       expense.ID,
		"user_id":                  expense.UserID,
		"name":                     expense.Name,
		"amount":                   expense.Amount,
		"currency":                 expense.Currency,
		"expense_type":             expense.ExpenseType,
		"tag_id":                   expense.TagID,
		"expense_date":             expense.ExpenseDate,
		"period_year":              expense.PeriodYear,
		"period_month":             expense.PeriodMonth,
		"status":                   expense.Status,
		"corrects_id":              expense.CorrectsID,
		"is_pro_rata":              expense.IsProRata,
		"pro_rata_group":           expense.ProRataGroup,
		"pro_rata_index":           expense.ProRataIndex,
		"pro_rata_total":           expense.ProRataTotal,
		"created_at":               expense.CreatedAt,
		"transaction_amount":       expense.TransactionAmount,
		"transaction_currency":     expense.TransactionCurrency,
		"reporting_amount":         expense.ReportingAmount,
		"reporting_currency":       expense.ReportingCurrency,
		"exchange_rate":            expense.ExchangeRate,
		"exchange_rate_source":     expense.ExchangeRateSource,
		"exchange_rate_timestamp":  expense.ExchangeRateTimestamp,
		"exchange_rate_expires_at": expense.ExchangeRateExpiresAt,
	}

	_, err := r.client.SQLExec(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("inserting expense: %w", err)
	}

	return expense, nil
}

// GetExpensesForPeriod returns materialized (active-only) expenses for the given
// user and period, with pagination. Also returns the total count.
func (r *ImmudbExpenseRepository) GetExpensesForPeriod(ctx context.Context, userID string, year, month, page, pageSize int32) ([]*model.Expense, int64, error) {
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
		expense, convErr := rowToExpense(row)
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

	expense, err := rowToExpense(result.Rows[0])
	if err != nil {
		return nil, fmt.Errorf("mapping expense row: %w", err)
	}
	return expense, nil
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

const expenseSuggestionInputLimit int32 = 1000

// GetActiveExpenseSuggestionInputs returns recent active expense rows for suggestion ranking.
func (r *ImmudbExpenseRepository) GetActiveExpenseSuggestionInputs(ctx context.Context, userID string) ([]*model.ExpenseSuggestionInput, error) {
	query := `SELECT id, name, amount, currency, expense_type, tag_id, created_at,
		expense_date, is_pro_rata, pro_rata_group
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

// expenseColumnCount is the number of columns rowToExpense expects in a result
// row. It must match the expenseSelectColumns list and the ExpenseData schema.
const expenseColumnCount = 25

// rowToExpense maps a result row to an Expense. The column order must match the
// SELECT clause in queries. It returns an error on a short/malformed row rather
// than panicking on an out-of-range index.
//
// Every row must carry the required snapshot fields. InitSchema backfills rows
// missing them at startup, so a row still missing a required field on read is a
// data-integrity error.
func rowToExpense(row SQLRow) (*model.Expense, error) {
	values := row.Values
	if len(values) < expenseColumnCount {
		return nil, fmt.Errorf("expense row has %d values, want %d", len(values), expenseColumnCount)
	}
	exp := &model.Expense{
		ID:                    values[0].GetString(),
		UserID:                values[1].GetString(),
		Name:                  values[2].GetString(),
		Amount:                values[3].GetInt(),
		Currency:              values[4].GetString(),
		ExpenseType:           values[5].GetString(),
		TagID:                 values[6].GetString(),
		ExpenseDate:           values[7].GetString(),
		PeriodYear:            int32(values[8].GetInt()),
		PeriodMonth:           int32(values[9].GetInt()),
		Status:                values[10].GetString(),
		CorrectsID:            values[11].GetString(),
		IsProRata:             values[12].GetBool(),
		ProRataGroup:          values[13].GetString(),
		ProRataIndex:          int32(values[14].GetInt()),
		ProRataTotal:          int32(values[15].GetInt()),
		CreatedAt:             values[16].GetString(),
		TransactionAmount:     values[17].GetInt(),
		TransactionCurrency:   values[18].GetString(),
		ReportingAmount:       values[19].GetInt(),
		ReportingCurrency:     values[20].GetString(),
		ExchangeRate:          values[21].GetString(),
		ExchangeRateSource:    values[22].GetString(),
		ExchangeRateTimestamp: values[23].GetString(),
		ExchangeRateExpiresAt: values[24].GetString(),
	}

	if exp.TransactionAmount == 0 || exp.TransactionCurrency == "" ||
		exp.ReportingAmount == 0 || exp.ReportingCurrency == "" ||
		exp.ExchangeRate == "" || exp.ExchangeRateSource == "" ||
		exp.ExchangeRateTimestamp == "" {
		return nil, fmt.Errorf("expense row %s: missing required snapshot fields", exp.ID)
	}

	return exp, nil
}

// Column order must match GetActiveExpenseSuggestionInputs SELECT clause.
func rowToExpenseSuggestionInput(row SQLRow) *model.ExpenseSuggestionInput {
	values := row.Values
	return &model.ExpenseSuggestionInput{
		ID:           values[0].GetString(),
		Name:         values[1].GetString(),
		Amount:       values[2].GetInt(),
		Currency:     values[3].GetString(),
		ExpenseType:  values[4].GetString(),
		TagID:        values[5].GetString(),
		CreatedAt:    values[6].GetString(),
		ExpenseDate:  values[7].GetString(),
		IsProRata:    values[8].GetBool(),
		ProRataGroup: values[9].GetString(),
	}
}

// CorrectExpense atomically marks the original expense as "corrected" and
// inserts a new correction entry. immudb's SQL interface does not support
// multi-statement ExecAll; each statement is individually atomic in its MVCC
// model. For a single-user personal finance app, sequential execution is safe.
func (r *ImmudbExpenseRepository) CorrectExpense(ctx context.Context, original *model.Expense, correction *model.Expense) (*model.Expense, error) {
	updateQuery := `UPDATE expenses SET status = 'corrected' WHERE id = @id;`
	_, err := r.client.SQLExec(ctx, updateQuery, map[string]interface{}{
		"id": original.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marking original expense as corrected: %w", err)
	}

	created, err := r.CreateExpense(ctx, correction)
	if err != nil {
		return nil, fmt.Errorf("inserting correction entry: %w", err)
	}

	r.logger.Info("expense corrected",
		slog.String("original_id", original.ID),
		slog.String("correction_id", created.ID),
	)

	return created, nil
}

// expenseSelectColumns is the shared SELECT column list for expense queries.
const expenseSelectColumns = `id, user_id, name, amount, currency, expense_type, tag_id,
		expense_date, period_year, period_month, status, corrects_id,
		is_pro_rata, pro_rata_group, pro_rata_index, pro_rata_total, created_at,
		transaction_amount, transaction_currency,
		reporting_amount, reporting_currency, exchange_rate, exchange_rate_source,
		exchange_rate_timestamp, exchange_rate_expires_at`

// GetCorrectionHistory returns the full correction chain for an expense.
// It finds the root of the chain (the original entry with no corrects_id),
// then collects all entries that form the chain in chronological order.
func (r *ImmudbExpenseRepository) GetCorrectionHistory(ctx context.Context, expenseID string, userID string) ([]*model.Expense, error) {
	// First, fetch the requested expense
	starting, err := r.GetExpenseByID(ctx, expenseID, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching starting expense: %w", err)
	}
	if starting == nil {
		return nil, nil
	}

	// Walk backwards to the root (original) via corrects_id
	root := starting
	visited := map[string]bool{root.ID: true}
	for root.CorrectsID != "" {
		parent, parentErr := r.GetExpenseByID(ctx, root.CorrectsID, userID)
		if parentErr != nil {
			return nil, fmt.Errorf("walking correction chain: %w", parentErr)
		}
		if parent == nil {
			break
		}
		if visited[parent.ID] {
			break // Safety: prevent infinite loops
		}
		visited[parent.ID] = true
		root = parent
	}

	// Now collect the chain from root forward:
	// root -> correction1 -> correction2 -> ...
	chain := []*model.Expense{root}
	currentID := root.ID

	for {
		// Find the entry that corrects the current one
		nextQuery := fmt.Sprintf(`SELECT %s FROM expenses
			WHERE corrects_id = @corrects_id AND user_id = @user_id;`, expenseSelectColumns)

		result, queryErr := r.client.SQLQuery(ctx, nextQuery, map[string]interface{}{
			"corrects_id": currentID,
			"user_id":     userID,
		})
		if queryErr != nil {
			return nil, fmt.Errorf("following correction chain forward: %w", queryErr)
		}

		if len(result.Rows) == 0 {
			break
		}

		next, convErr := rowToExpense(result.Rows[0])
		if convErr != nil {
			return nil, fmt.Errorf("mapping expense row: %w", convErr)
		}
		if visited[next.ID] {
			break // Safety
		}
		visited[next.ID] = true
		chain = append(chain, next)
		currentID = next.ID
	}

	return chain, nil
}

// GetProRataGroup returns all expenses in a pro-rata group for a user,
// ordered by installment index.
func (r *ImmudbExpenseRepository) GetProRataGroup(ctx context.Context, groupID string, userID string) ([]*model.Expense, error) {
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
		expense, convErr := rowToExpense(row)
		if convErr != nil {
			return nil, fmt.Errorf("mapping expense row: %w", convErr)
		}
		expenses = append(expenses, expense)
	}

	return expenses, nil
}

// GetExpensesByUserAfter returns one keyset page of expenses (active +
// corrected) for a user past the given cursor, ordered by
// (created_at ASC, id ASC). It seeks with the expanded-OR (created_at, id)
// predicate instead of LIMIT/OFFSET and derives hasMore by fetching pageSize+1
// rows and inspecting the overflow row, so it issues no OFFSET and no per-page
// COUNT(*). An empty cursor starts from the beginning.
//
// immudb 1.11.0 does not support SQL row-value tuple syntax
// ((created_at, id) > (@c, @cid) raises a syntax error), so the comparison is
// written in expanded-OR form.
func (r *ImmudbExpenseRepository) GetExpensesByUserAfter(ctx context.Context, userID string, cursor ExpenseCursor, pageSize int32) ([]*model.Expense, ExpenseCursor, bool, error) {
	if pageSize < 1 {
		pageSize = DefaultStreamPageSize
	}

	// Fetch one extra row (pageSize+1) so the overflow row reveals whether more
	// rows remain, avoiding a per-page COUNT(*).
	params := map[string]interface{}{
		"user_id": userID,
		"limit":   pageSize + 1,
	}

	cursorPredicate := ""
	if cursor.CreatedAt != "" {
		cursorPredicate = ` AND (created_at > @cursor_created_at
		OR (created_at = @cursor_created_at AND id > @cursor_id))`
		params["cursor_created_at"] = cursor.CreatedAt
		params["cursor_id"] = cursor.ID
	}

	dataQuery := fmt.Sprintf(`SELECT %s FROM expenses
		WHERE user_id = @user_id%s
		ORDER BY created_at ASC, id ASC
		LIMIT @limit;`, expenseSelectColumns, cursorPredicate)

	result, err := r.client.SQLQuery(ctx, dataQuery, params)
	if err != nil {
		return nil, ExpenseCursor{}, false, fmt.Errorf("querying user expenses after cursor: %w", err)
	}

	rows := make([]*model.Expense, 0, len(result.Rows))
	for _, row := range result.Rows {
		expense, convErr := rowToExpense(row)
		if convErr != nil {
			return nil, ExpenseCursor{}, false, fmt.Errorf("mapping expense row: %w", convErr)
		}
		rows = append(rows, expense)
	}

	// The overflow row (pageSize+1th) means more rows remain. Drop it from the
	// page and report hasMore.
	hasMore := int32(len(rows)) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	next := cursor
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		next = ExpenseCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	return rows, next, hasMore, nil
}

// AnonymizeAllUserExpenses redacts PII fields on all expense rows for a user.
// The UPDATE overwrites the current head in immudb's append-only model.
// Idempotent: re-calling for already-redacted data produces the same result.
// Returns nil when zero rows match (user has no expenses).
func (r *ImmudbExpenseRepository) AnonymizeAllUserExpenses(ctx context.Context, userID string) error {
	query := `UPDATE expenses SET
		user_id = 'DELETED',
		name = 'REDACTED',
		amount = 0,
		currency = '',
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
