package model

// Finance-specific error codes. The shared, cross-service codes
// (VALIDATION_ERROR, UNAUTHORIZED, NOT_FOUND, INTERNAL_SERVER_ERROR) are
// single-sourced in the apierr module; only codes unique to finance are
// declared here. They remain valid apierr.Error Code strings.
const (
	ErrPeriodNotFound             = "PERIOD_NOT_FOUND"
	ErrDuplicateTag               = "DUPLICATE_TAG"
	ErrTagInUse                   = "TAG_IN_USE"
	ErrDefaultTag                 = "DEFAULT_TAG"
	ErrPeriodLocked               = "PERIOD_LOCKED"
	ErrUnsupportedCurrency        = "UNSUPPORTED_CURRENCY"
	ErrReportingCurrencyImmutable = "REPORTING_CURRENCY_IMMUTABLE"
	ErrConversionUnavailable      = "CONVERSION_UNAVAILABLE"
	// ErrSnapshotCurrencyMissing is returned when a captured pro-rata snapshot
	// lacks a rate needed to derive a target-period reporting amount. The
	// schedule moves to failed and no ledger row is written.
	ErrSnapshotCurrencyMissing = "SNAPSHOT_CURRENCY_MISSING"
	// ErrMissingCapturedRateSnapshot is returned when a legacy pending
	// pro-rata schedule has no captured snapshot and the target period
	// reporting currency differs from the stored schedule currency. The
	// schedule moves to failed and no ledger row is written.
	ErrMissingCapturedRateSnapshot = "MISSING_CAPTURED_RATE_SNAPSHOT"
)
