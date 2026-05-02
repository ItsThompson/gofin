package service

import "context"

// ExpenseData holds the fields the finance service needs from an expense record.
// This is a service-level DTO that decouples the aggregation logic from the
// gRPC protobuf types.
type ExpenseData struct {
	ID          string
	Amount      int64
	ExpenseType string // "essentials", "desires", "savings"
	TagID       string
	ExpenseDate string // "YYYY-MM-DD"
}

// ExpenseClient abstracts the gRPC call to the expense service.
// The finance service uses this to fetch expense data for aggregation.
// In production, this wraps a gRPC client. In tests, it's a mock.
type ExpenseClient interface {
	// GetExpensesForPeriod returns all active expenses for a user's budget period.
	// The returned slice contains every active expense (no pagination): the finance
	// service needs all expenses to compute accurate aggregations.
	GetExpensesForPeriod(ctx context.Context, userID string, year, month int32) ([]ExpenseData, error)
}
