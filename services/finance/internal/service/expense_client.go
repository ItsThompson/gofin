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
	CreateExpense(ctx context.Context, req CreateExpenseInput) (*CreatedExpenseData, error)
}

// CreateExpenseInput is the data needed to create an expense via the expense service.
type CreateExpenseInput struct {
	UserID       string
	Name         string
	Amount       int64
	Currency     string
	ExpenseType  string
	TagID        string
	ExpenseDate  string
	PeriodYear   int32
	PeriodMonth  int32
	IsProRata    bool
	ProRataGroup string
	ProRataIndex int32
	ProRataTotal int32
}

// CreatedExpenseData is the data returned after creating an expense.
type CreatedExpenseData struct {
	ID          string
	CreatedAt   string
}
