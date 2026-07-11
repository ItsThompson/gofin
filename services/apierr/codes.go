// Package apierr single-sources the GoFin API error contract: the typed
// service error, the shared error-code constants, and the HTTP wire-response
// mapping. It is the one place the {code, message, fields?} contract lives so
// all four API services emit a byte-identical error shape.
package apierr

// Shared, cross-service error codes (domain truth). Service-specific codes
// (e.g. PERIOD_LOCKED, DUPLICATE_TAG) stay as local constants in each service;
// they are still valid Code strings and do not belong here.
const (
	CodeUnauthorized = "UNAUTHORIZED"
	CodeNotFound     = "NOT_FOUND"
	CodeValidation   = "VALIDATION_ERROR"
	CodeInternal     = "INTERNAL_SERVER_ERROR"
	CodeForbidden    = "FORBIDDEN"
)
