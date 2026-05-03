package service

import "context"

// ExpenseData holds the fields the finance service needs from an expense record.
type ExpenseData struct {
	ID          string
	Amount      int64
	ExpenseType string // "essentials", "desires", "savings"
	TagID       string
	ExpenseDate string // "YYYY-MM-DD"
}

// ExpenseClient abstracts the gRPC call to the expense service.
// In production, this wraps a gRPC client. In tests, it's a mock.
type ExpenseClient interface {
	GetExpensesForPeriod(ctx context.Context, userID string, year, month int32) ([]ExpenseData, error)
	CountExpensesByTag(ctx context.Context, userID, tagID string) (int64, error)
}
