package model

// Expense represents an entry in the immutable expense ledger.
type Expense struct {
	ID                  string `json:"id"`
	UserID              string `json:"userId"`
	Name                string `json:"name"`
	Amount              int64  `json:"amount"` // Minor units (cents)
	TransactionCurrency string `json:"transactionCurrency"`
	Currency            string `json:"currency"`    // ISO 4217 code
	ExpenseType         string `json:"expenseType"` // "essentials", "desires", "savings"
	TagID               string `json:"tagId"`
	ExpenseDate         string `json:"expenseDate"` // ISO date: "2026-05-03"
	PeriodYear          int32  `json:"periodYear"`
	PeriodMonth         int32  `json:"periodMonth"`
	Status              string `json:"status"` // "active" or "corrected"
	CorrectsID          string `json:"correctsId,omitempty"`
	IsProRata           bool   `json:"isProRata"`
	ProRataGroup        string `json:"proRataGroup,omitempty"`
	ProRataIndex        int32  `json:"proRataIndex,omitempty"`
	ProRataTotal        int32  `json:"proRataTotal,omitempty"`
	CreatedAt           string `json:"createdAt"` // ISO 8601 timestamp
}

// ValidExpenseTypes is the set of allowed expense_type values.
var ValidExpenseTypes = map[string]bool{
	"essentials": true,
	"desires":    true,
	"savings":    true,
}
