package model

// Expense represents an entry in the immutable ledger.
type Expense struct {
	ID                      string `json:"id"`
	UserID                  string `json:"userId"`
	Name                    string `json:"name"`
	TransactionCurrencyCode string `json:"transactionCurrencyCode"`
	ExpenseType             string `json:"expenseType"` // "essentials", "desires", "savings"
	TagID                   string `json:"tagId"`
	ExpenseDateIso          string `json:"expenseDateIso"`
	PeriodYear              int32  `json:"periodYear"`
	PeriodMonth             int32  `json:"periodMonth"`
	Status                  string `json:"status"` // "active" or "corrected"
	CorrectsID              string `json:"correctsId,omitempty"`
	IsProRata               bool   `json:"isProRata"`
	ProRataGroup            string `json:"proRataGroup,omitempty"`
	ProRataIndex            int32  `json:"proRataIndex,omitempty"`
	ProRataTotal            int32  `json:"proRataTotal,omitempty"`
	CreatedAt               string `json:"createdAt"` // ISO 8601 timestamp

	// Money snapshot fields.
	OriginalTransactionAmountInMinorUnits int64  `json:"originalTransactionAmountInMinorUnits"`
	ReportingAmountInMinorUnits           int64  `json:"reportingAmountInMinorUnits"`
	ReportingCurrencyCode                 string `json:"reportingCurrencyCode"`
	SourceToTargetExchangeRate            string `json:"sourceToTargetExchangeRate"`
	ExchangeRateSource                    string `json:"exchangeRateSource"`
	ExchangeRateTimestamp                 string `json:"exchangeRateTimestamp"`
	// Present for live provider snapshots with cache expiry metadata.
	ExchangeRateCacheExpiresAt string `json:"exchangeRateCacheExpiresAt,omitempty"`

	// ClientGeneratedIdempotencyKey makes create idempotent: a retry with the
	// same key returns the already-created expense instead of inserting a
	// duplicate. Required on create (RFC 4122 UUID); empty for rows written
	// before the idempotency migration.
	ClientGeneratedIdempotencyKey string `json:"clientGeneratedIdempotencyKey"`
}

// ValidExpenseTypes is the set of allowed expense_type values.
var ValidExpenseTypes = map[string]bool{
	"essentials": true,
	"desires":    true,
	"savings":    true,
}
