package model

// Auth-specific error codes. The cross-service codes (UNAUTHORIZED, NOT_FOUND,
// VALIDATION_ERROR, INTERNAL_SERVER_ERROR, FORBIDDEN) live in the shared apierr
// package; only codes unique to the auth domain remain here. They are still
// valid apierr.Error Code strings.
const (
	ErrDuplicateEmail     = "DUPLICATE_EMAIL"
	ErrDuplicateUsername  = "DUPLICATE_USERNAME"
	ErrInvalidCredentials = "INVALID_CREDENTIALS"
	ErrWeakPassword       = "WEAK_PASSWORD"
)
