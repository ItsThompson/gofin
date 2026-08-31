package model

// Expense-specific error codes. The shared codes (VALIDATION_ERROR,
// UNAUTHORIZED, NOT_FOUND, INTERNAL_SERVER_ERROR) are single-sourced in the
// shared apierr package; reference them from there.
const (
	ErrAlreadyCorrected    = "ALREADY_CORRECTED"
	ErrPeriodLocked        = "PERIOD_LOCKED"
	ErrPeriodNotFound      = "PERIOD_NOT_FOUND"
	ErrUnsupportedCurrency = "UNSUPPORTED_CURRENCY"
	// ErrConversionUnavailable is returned when a foreign-currency expense cannot
	// be converted because the FX provider is unavailable (not yet wired or down).
	// No ledger row is written. Mapped to HTTP 503 / gRPC codes.Unavailable.
	ErrConversionUnavailable = "CONVERSION_UNAVAILABLE"
	// ErrSnapshotCurrencyMissing is returned when a captured pro-rata snapshot
	// lacks a rate needed to derive an installment reporting amount.
	ErrSnapshotCurrencyMissing = "SNAPSHOT_CURRENCY_MISSING"
)
