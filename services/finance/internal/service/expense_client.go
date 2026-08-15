package service

import (
	"context"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// ExpenseData holds the fields the finance service needs from an expense record.
type ExpenseData struct {
	ID                string
	ReportingAmount   int64  // Converted amount in the period reporting currency minor units.
	ReportingCurrency string // Budget period reporting currency for this row.
	ExpenseType       string // "essentials", "desires", "savings"
	TagID             string
	ExpenseDate       string // "YYYY-MM-DD"
}

// ExpenseClient abstracts the gRPC call to the expense service.
// In production, this wraps a gRPC client. In tests, it's a mock.
type ExpenseClient interface {
	GetExpensesForPeriod(ctx context.Context, userID string, year, month int32) ([]ExpenseData, error)
	CountExpensesByTag(ctx context.Context, userID, tagID string) (int64, error)
	CreateExpense(ctx context.Context, req CreateExpenseInput) (*CreatedExpenseData, error)
	CreateProRataInstallment(ctx context.Context, req CreateProRataInstallmentInput) (*CreatedExpenseData, error)
}

// TrustedPeriodContext is period context Finance resolved locally before asking
// Expense to write a pro-rata installment. Expense validates it but does not call
// Finance again for Finance-originated writes.
type TrustedPeriodContext struct {
	PeriodID          string
	UserID            string
	Year              int32
	Month             int32
	ReportingCurrency string
	Source            string
}

// CreateProRataInstallmentInput is the Finance-originated internal write contract.
type CreateProRataInstallmentInput struct {
	UserID               string
	PeriodContext        TrustedPeriodContext
	Name                 string
	Amount               int64
	Currency             string
	ExpenseType          string
	TagID                string
	ExpenseDate          string
	ProRataGroup         string
	ProRataIndex         int32
	ProRataTotal         int32
	CapturedRateSnapshot *model.CapturedRateSnapshot
}

// CreateExpenseInput is the data needed to create an expense via the expense service.
type CreateExpenseInput struct {
	UserID              string
	Name                string
	Amount              int64
	TransactionCurrency string
	ExpenseType         string
	TagID               string
	ExpenseDate         string
	PeriodYear          int32
	PeriodMonth         int32
	IsProRata           bool
	ProRataGroup        string
	ProRataIndex        int32
	ProRataTotal        int32
}

// CreatedExpenseData is the data returned after creating an expense.
type CreatedExpenseData struct {
	ID        string
	CreatedAt string
}
