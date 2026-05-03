package repository

import (
	"context"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// ExpenseRepository defines the data access contract for expense operations.
// Implementations can be backed by immudb (production) or mocks (tests).
type ExpenseRepository interface {
	// CreateExpense inserts a new expense entry into the ledger.
	CreateExpense(ctx context.Context, expense *model.Expense) (*model.Expense, error)

	// GetExpensesForPeriod returns materialized (status=active only) expenses
	// for the given user and period, ordered by expense_date DESC, created_at DESC.
	// Returns the matching expenses and the total count for pagination.
	GetExpensesForPeriod(ctx context.Context, userID string, year, month, page, pageSize int32) ([]*model.Expense, int64, error)

	// GetExpenseByID returns a single expense by ID, scoped to the given user.
	// Returns nil if the expense doesn't exist or belongs to a different user.
	GetExpenseByID(ctx context.Context, id string, userID string) (*model.Expense, error)

	// CountExpensesByTag returns the number of active expenses referencing the given tag.
	CountExpensesByTag(ctx context.Context, userID string, tagID string) (int64, error)

	// CorrectExpense atomically marks the original expense as "corrected" and
	// inserts a new correction entry with status "active". Returns the new entry.
	CorrectExpense(ctx context.Context, original *model.Expense, correction *model.Expense) (*model.Expense, error)

	// GetCorrectionHistory returns the full correction chain for an expense,
	// ordered chronologically (original first, latest correction last).
	GetCorrectionHistory(ctx context.Context, expenseID string, userID string) ([]*model.Expense, error)

	// GetProRataGroup returns all expenses belonging to a pro-rata group,
	// scoped to a user.
	GetProRataGroup(ctx context.Context, groupID string, userID string) ([]*model.Expense, error)
}

// SchemaInitializer creates the required tables and indexes on startup.
type SchemaInitializer interface {
	InitSchema(ctx context.Context) error
}

// SQLResult represents the result of an SQL query row.
type SQLResult struct {
	Columns []string
	Rows    []SQLRow
}

// SQLRow represents a single row from an SQL query result.
type SQLRow struct {
	Values []SQLValue
}

// SQLValue represents a single value in a row. The immudb client returns
// typed values; this interface abstracts over them for testability.
type SQLValue interface {
	GetString() string
	GetInt() int64
	GetBool() bool
}

// ImmudbClient abstracts the immudb client operations used by the repository.
// This thin interface decouples the repository from the concrete immudb client
// import, making the code buildable without the immudb dependency (which is
// resolved at container build time via Docker).
type ImmudbClient interface {
	SQLExec(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error)
	SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error)
}
