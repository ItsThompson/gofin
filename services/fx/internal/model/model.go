package model

type ErrorCode string

const (
	ErrorUnsupportedCurrency      ErrorCode = "UNSUPPORTED_CURRENCY"
	ErrorInvalidAmount            ErrorCode = "INVALID_AMOUNT"
	ErrorConversionUnavailable    ErrorCode = "CONVERSION_UNAVAILABLE"
	ErrorProviderAuthFailed       ErrorCode = "PROVIDER_AUTH_FAILED"
	ErrorProviderResponseInvalid  ErrorCode = "PROVIDER_RESPONSE_INVALID"
	ErrorRateMissing              ErrorCode = "RATE_MISSING"
	ErrorSnapshotIntegrityFailure ErrorCode = "SNAPSHOT_INTEGRITY_FAILURE"
)

const (
	BaseCurrencyUSD     = "USD"
	CacheStatusHit      = "hit"
	CacheStatusMiss     = "miss"
	CacheStatusProvided = "provided_snapshot"
	SnapshotVersion     = 1
)

type Error struct {
	Code  ErrorCode
	Field string
	Err   error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return string(e.Code) + ": " + e.Err.Error()
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewError(code ErrorCode, field string, err error) *Error {
	return &Error{Code: code, Field: field, Err: err}
}

type ProviderSnapshot struct {
	Source        string
	BaseCurrency  string
	RateTimestamp string
	CapturedAt    string
	ExpiresAt     string
	Rates         map[string]string
}

type CapturedRateSnapshot struct {
	SnapshotVersion int32
	Source          string
	BaseCurrency    string
	RateTimestamp   string
	CapturedAt      string
	ExpiresAt       string
	RatesByCurrency map[string]string
}

type SnapshotResult struct {
	Snapshot    CapturedRateSnapshot
	CacheStatus string
}

type ConvertRequest struct {
	Amount         int64
	SourceCurrency string
	TargetCurrency string
	RequestedAt    string
}

type ConvertWithSnapshotRequest struct {
	Amount         int64
	SourceCurrency string
	TargetCurrency string
	RequestedAt    string
	Snapshot       CapturedRateSnapshot
}

type ConvertResponse struct {
	ConvertedAmount int64
	SourceCurrency  string
	TargetCurrency  string
	ExchangeRate    string
	RateTimestamp   string
	Source          string
	CacheStatus     string
	ExpiresAt       string
}
