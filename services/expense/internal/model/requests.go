package model

// CreateExpenseRequest is the input for POST /api/expenses.
// Note: binding tags are intentionally omitted. Validation is handled by the
// service layer's validateCreateExpenseRequest, which returns field-level error
// details in the error response.
type CreateExpenseRequest struct {
	Name                string `json:"name"`
	Amount              int64  `json:"amount"`
	TransactionCurrency string `json:"transactionCurrency"`
	ExpenseType         string `json:"expenseType"`
	TagID               string `json:"tagId"`
	ExpenseDate         string `json:"expenseDate"`
	PeriodYear          int32  `json:"periodYear"`
	PeriodMonth         int32  `json:"periodMonth"`

	// Pro-rata fields (optional for standard expenses)
	IsProRata    bool   `json:"isProRata,omitempty"`
	ProRataGroup string `json:"proRataGroup,omitempty"`
	ProRataIndex int32  `json:"proRataIndex,omitempty"`
	ProRataTotal int32  `json:"proRataTotal,omitempty"`

	// ClientGeneratedIdempotencyKey makes create idempotent: a retry with the
	// same key returns the already-created expense instead of inserting a
	// duplicate. Required (RFC 4122 UUID).
	ClientGeneratedIdempotencyKey string `json:"clientGeneratedIdempotencyKey"`
}

// GetExpensesRequest holds the parsed query parameters for GET /api/expenses.
// The repository query is scoped to user_id + period and ordered by
// expense_date DESC; all further filtering and sorting is done client-side, so
// no sort/type/tag/date-range fields are carried here.
type GetExpensesRequest struct {
	UserID   string
	Year     int32
	Month    int32
	Page     int32
	PageSize int32
}

// ExpenseResponse is the JSON body returned for a single expense.
type ExpenseResponse struct {
	Expense *Expense `json:"expense"`
}

// ExpenseListResponse is the paginated response for listing expenses.
type ExpenseListResponse struct {
	Data     []*Expense `json:"data"`
	Total    int64      `json:"total"`
	Page     int32      `json:"page"`
	PageSize int32      `json:"pageSize"`
	HasMore  bool       `json:"hasMore"`
}

// CorrectExpenseRequest is the input for POST /api/expenses/:id/correct.
type CorrectExpenseRequest struct {
	Name                string `json:"name"`
	Amount              int64  `json:"amount"`
	TransactionCurrency string `json:"transactionCurrency"`
	ExpenseType         string `json:"expenseType"`
	TagID               string `json:"tagId"`
	ExpenseDate         string `json:"expenseDate"`
}

// CorrectionHistoryResponse is the response for GET /api/expenses/:id/history.
// Entries are ordered chronologically: original first, latest correction last.
type CorrectionHistoryResponse struct {
	Entries []*Expense `json:"entries"`
}

// ProRataGroupResponse is the response for GET /api/expenses/prorata/:groupId.
type ProRataGroupResponse struct {
	Expenses []*Expense `json:"expenses"`
}
