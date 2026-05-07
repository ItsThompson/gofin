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
	ErrDuplicateEmail      = "DUPLICATE_EMAIL"
	ErrDuplicateUsername   = "DUPLICATE_USERNAME"
	ErrInvalidCredentials  = "INVALID_CREDENTIALS"
	ErrWeakPassword        = "WEAK_PASSWORD"
	ErrValidationError     = "VALIDATION_ERROR"
	ErrUnauthorized        = "UNAUTHORIZED"
	ErrForbidden           = "FORBIDDEN"
	ErrNotFound            = "NOT_FOUND"
	ErrProtectedUser       = "PROTECTED_USER"
	ErrInternalServerError = "INTERNAL_SERVER_ERROR"
)
