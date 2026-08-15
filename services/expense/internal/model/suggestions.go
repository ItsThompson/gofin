package model

// ExpenseSuggestionRequest holds parsed query parameters for ranked suggestions.
type ExpenseSuggestionRequest struct {
	UserID   string
	Page     int32
	PageSize int32
}

// ExpenseSuggestionInput is the minimal active row data required to build suggestions.
// TransactionAmount/TransactionCurrency come from the explicit money snapshot
// columns. Legacy rows without snapshot columns fall back to Amount/Currency so
// suggestions remain correct during the rollout window.
type ExpenseSuggestionInput struct {
	ID                  string
	Name                string
	Amount              int64  // Legacy minor-units column; mirrors TransactionAmount for new rows.
	Currency            string // Legacy ISO 4217 column; mirrors TransactionCurrency for new rows.
	TransactionAmount   int64  // Original transaction amount in minor units from the money snapshot.
	TransactionCurrency string // Original transaction currency from the money snapshot.
	ExpenseType         string
	TagID               string
	CreatedAt           string
	ExpenseDate         string
	IsProRata           bool
	ProRataGroup        string
}

// ExpenseSuggestion is one aggregated exact-name suggestion.
// TransactionAmount/TransactionCurrency are the canonical fields sourced from
// the latest active matching expense. Amount/Currency are deprecated aliases
// that mirror the transaction values for rollout compatibility with older
// frontend clients.
type ExpenseSuggestion struct {
	Name                string  `json:"name"`
	TransactionAmount   int64   `json:"transactionAmount"`
	TransactionCurrency string  `json:"transactionCurrency"`
	Amount              int64   `json:"amount"`   // Deprecated: mirrors TransactionAmount.
	Currency            string  `json:"currency"` // Deprecated: mirrors TransactionCurrency.
	ExpenseType         string  `json:"expenseType"`
	TagID               string  `json:"tagId"`
	Frequency           int32   `json:"frequency"`
	LastUsedAt          string  `json:"lastUsedAt"`
	RecencyBucket       string  `json:"recencyBucket"`
	FrecencyScore       float64 `json:"frecencyScore"`
}

// ExpenseSuggestionListResponse follows the app pagination shape.
type ExpenseSuggestionListResponse struct {
	Data     []*ExpenseSuggestion `json:"data"`
	Total    int64                `json:"total"`
	Page     int32                `json:"page"`
	PageSize int32                `json:"pageSize"`
	HasMore  bool                 `json:"hasMore"`
}
