package model

// ApiError follows the error contract from 10-nonfunctional.md:
// { code, message, fields? }
type ApiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Common error codes
const (
	ErrValidationError     = "VALIDATION_ERROR"
	ErrUnauthorized        = "UNAUTHORIZED"
	ErrNotFound            = "NOT_FOUND"
	ErrInternalServerError = "INTERNAL_SERVER_ERROR"
	ErrAlreadyCorrected    = "ALREADY_CORRECTED"
	ErrPeriodLocked        = "PERIOD_LOCKED"
)
