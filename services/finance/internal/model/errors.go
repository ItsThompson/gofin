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
)
