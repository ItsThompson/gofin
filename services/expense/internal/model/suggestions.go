package model

// ExpenseSuggestionRequest holds parsed query parameters for ranked suggestions.
type ExpenseSuggestionRequest struct {
	UserID   string
	Page     int32
	PageSize int32
}

// ExpenseSuggestionInput is the minimal active row data required to build suggestions.
type ExpenseSuggestionInput struct {
	ID           string
	Name         string
	Amount       int64
	Currency     string
	ExpenseType  string
	TagID        string
	CreatedAt    string
	ExpenseDate  string
	IsProRata    bool
	ProRataGroup string
}

// ExpenseSuggestion is one aggregated exact-name suggestion.
type ExpenseSuggestion struct {
	Name          string  `json:"name"`
	Amount        int64   `json:"amount"`
	Currency      string  `json:"currency"`
	ExpenseType   string  `json:"expenseType"`
	TagID         string  `json:"tagId"`
	Frequency     int32   `json:"frequency"`
	LastUsedAt    string  `json:"lastUsedAt"`
	RecencyBucket string  `json:"recencyBucket"`
	FrecencyScore float64 `json:"frecencyScore"`
}

// ExpenseSuggestionListResponse follows the app pagination shape.
type ExpenseSuggestionListResponse struct {
	Data     []*ExpenseSuggestion `json:"data"`
	Total    int64                `json:"total"`
	Page     int32                `json:"page"`
	PageSize int32                `json:"pageSize"`
	HasMore  bool                 `json:"hasMore"`
}
