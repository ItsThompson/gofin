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

	// GetExpenseByID returns a single expense by ID (active only).
	GetExpenseByID(ctx context.Context, id string) (*model.Expense, error)
}

// SchemaInitializer creates the required tables and indexes on startup.
type SchemaInitializer interface {
	InitSchema(ctx context.Context) error
}
