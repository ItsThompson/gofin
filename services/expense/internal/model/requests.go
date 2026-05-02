package model

// CreateExpenseRequest is the input for POST /api/expenses.
type CreateExpenseRequest struct {
	Name        string `json:"name" binding:"required"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency" binding:"required"`
	ExpenseType string `json:"expenseType" binding:"required"`
	TagID       string `json:"tagId" binding:"required"`
	ExpenseDate string `json:"expenseDate" binding:"required"`
	PeriodYear  int32  `json:"periodYear" binding:"required"`
	PeriodMonth int32  `json:"periodMonth" binding:"required"`

	// Pro-rata fields (optional for standard expenses)
	IsProRata    bool   `json:"isProRata,omitempty"`
	ProRataGroup string `json:"proRataGroup,omitempty"`
	ProRataIndex int32  `json:"proRataIndex,omitempty"`
	ProRataTotal int32  `json:"proRataTotal,omitempty"`
}

// GetExpensesRequest holds the parsed query parameters for GET /api/expenses.
type GetExpensesRequest struct {
	UserID      string
	Year        int32
	Month       int32
	Page        int32
	PageSize    int32
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
