package model

import "time"

// ProRataSchedule represents a scheduled pro-rata installment in PostgreSQL.
type ProRataSchedule struct {
	ID                            string     `json:"id"`
	UserID                        string     `json:"userId"`
	ProRataGroup                  string     `json:"proRataGroup"`
	Name                          string     `json:"name"`
	InstallmentAmountInMinorUnits int64      `json:"installmentAmountInMinorUnits"`
	Currency                      string     `json:"currency"`
	ExpenseType                   string     `json:"expenseType"`
	TagID                         string     `json:"tagId"`
	TargetYear                    int32      `json:"targetYear"`
	TargetMonth                   int32      `json:"targetMonth"`
	InstallmentIndex              int32      `json:"installmentIndex"`
	InstallmentTotal              int32      `json:"installmentTotal"`
	Status                        string     `json:"status"`
	CreatedAt                     time.Time  `json:"createdAt"`
	AppliedAt                     *time.Time `json:"appliedAt"`

	// Capture intent fields for multi-currency pro-rata schedules.
	OriginalTransactionAmountInMinorUnits int64                 `json:"originalTransactionAmountInMinorUnits"`
	TransactionCurrencyCode               string                `json:"transactionCurrencyCode"`
	CreationReportingCurrency             string                `json:"creationReportingCurrency"`
	CapturedRateSnapshot                  *CapturedRateSnapshot `json:"capturedRateSnapshot,omitempty"`
	FailureReason                         string                `json:"failureReason,omitempty"`
}

// CapturedRateSnapshot is the USD-based provider snapshot stored on pro-rata
// schedule rows so future target periods can derive reporting amounts without
// a live provider rate.
type CapturedRateSnapshot struct {
	SnapshotVersion int32             `json:"snapshotVersion"`
	Source          string            `json:"source"`
	BaseCurrency    string            `json:"baseCurrency"`
	RateTimestamp   string            `json:"rateTimestamp"`
	CapturedAt      string            `json:"capturedAt"`
	ExpiresAt       string            `json:"expiresAt"`
	RatesByCurrency map[string]string `json:"ratesByCurrency"`
}

// CreateProRataRequest is the input for POST /api/finance/prorata.
type CreateProRataRequest struct {
	Name                    string `json:"name" binding:"required"`
	TotalAmountInMinorUnits int64  `json:"totalAmountInMinorUnits" binding:"required"`
	TransactionCurrencyCode string `json:"transactionCurrencyCode"`
	ExpenseType             string `json:"expenseType" binding:"required"`
	TagID                   string `json:"tagId" binding:"required"`
	ExpenseDateIso          string `json:"expenseDateIso" binding:"required"`
	SpreadOverMonths        int32  `json:"spreadOverMonths" binding:"required"`
	PeriodYear              int32  `json:"periodYear" binding:"required"`
	PeriodMonth             int32  `json:"periodMonth" binding:"required"`
}

// ProRataResponse is the JSON body returned for POST /api/finance/prorata.
type ProRataResponse struct {
	Expense   *CreatedExpense    `json:"expense"`
	Schedules []*ProRataSchedule `json:"schedules"`
}

// CreatedExpense is a simplified expense representation returned by the finance service
// after the expense service creates it via gRPC.
type CreatedExpense struct {
	ID                            string `json:"id"`
	Name                          string `json:"name"`
	InstallmentAmountInMinorUnits int64  `json:"installmentAmountInMinorUnits"`
	TransactionCurrencyCode       string `json:"transactionCurrencyCode"`
	Currency                      string `json:"currency"`
	ExpenseType                   string `json:"expenseType"`
	TagID                         string `json:"tagId"`
	ExpenseDateIso                string `json:"expenseDateIso"`
	PeriodYear                    int32  `json:"periodYear"`
	PeriodMonth                   int32  `json:"periodMonth"`
	IsProRata                     bool   `json:"isProRata"`
	ProRataGroup                  string `json:"proRataGroup"`
	ProRataIndex                  int32  `json:"proRataIndex"`
	ProRataTotal                  int32  `json:"proRataTotal"`
	CreatedAt                     string `json:"createdAt"`
}

// UpcomingProRataResponse is the JSON body returned for GET /api/finance/prorata/upcoming.
type UpcomingProRataResponse struct {
	Schedules []*ProRataSchedule `json:"schedules"`
}

// CreatePeriodResponse extends PeriodResponse with pro-rata application info.
type CreatePeriodResponse struct {
	Period             *BudgetPeriod      `json:"period"`
	AppliedProRata     []*ProRataSchedule `json:"appliedProRata"`
	AutoCreatedPeriods int                `json:"autoCreatedPeriods,omitempty"`
	AutoCreatedMonths  []string           `json:"autoCreatedMonths,omitempty"`
}
