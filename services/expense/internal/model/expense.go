package model

// Exchange rate snapshot sources.
const (
	ExchangeSourceIdentity          = "identity"
	ExchangeSourceOpenExchangeRates = "open_exchange_rates"
	ExchangeSourceMigration         = "migration"
)

// Expense represents an entry in the immutable expense ledger.
type Expense struct {
	ID                  string `json:"id"`
	UserID              string `json:"userId"`
	Name                string `json:"name"`
	Amount              int64  `json:"amount"` // Legacy minor-units column; equals TransactionAmount for new same-currency rows.
	TransactionCurrency string `json:"transactionCurrency"`
	Currency            string `json:"currency"`    // Legacy ISO 4217 column; mirrors TransactionCurrency during rollout.
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

	// Money snapshot fields.
	TransactionAmount     int64  `json:"transactionAmount"`
	ReportingAmount       int64  `json:"reportingAmount"`
	ReportingCurrency     string `json:"reportingCurrency"`
	ExchangeRate          string `json:"exchangeRate"`
	ExchangeRateSource    string `json:"exchangeRateSource"`
	ExchangeRateTimestamp string `json:"exchangeRateTimestamp"`
	ExchangeRateExpiresAt string `json:"exchangeRateExpiresAt,omitempty"`

	// LegacySynthesized is set by the repository when a row had no
	// money_snapshot_version (legacy) and the snapshot was synthesized. It is
	// not serialized (json:"-") and not mapped to proto. The service layer uses
	// it to apply period-context-aware currency normalization and telemetry.
	LegacySynthesized bool `json:"-"`

	// PartialSnapshotFields is set when a legacy row (null version) had some
	// nullable snapshot columns partially present from an incomplete migration
	// attempt. The service layer emits repair telemetry for these rows.
	PartialSnapshotFields bool `json:"-"`
}

// ValidExpenseTypes is the set of allowed expense_type values.
var ValidExpenseTypes = map[string]bool{
	"essentials": true,
	"desires":    true,
	"savings":    true,
}
