package model

// Expense-specific error codes. The shared codes (VALIDATION_ERROR,
// UNAUTHORIZED, NOT_FOUND, INTERNAL_SERVER_ERROR) are single-sourced in the
// shared apierr package; reference them from there.
const (
	ErrAlreadyCorrected = "ALREADY_CORRECTED"
	ErrPeriodLocked     = "PERIOD_LOCKED"
)
