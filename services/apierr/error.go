package apierr

import (
	"errors"
	"net/http"
)

// Error is the single typed service error. It carries everything the wire
// contract can express, including optional field-level validation detail (Fields).
type Error struct {
	Code    string            // maps to the "code" wire field
	Message string            // maps to "message"
	Status  int               // HTTP status to write
	Fields  map[string]string // optional field-level validation detail
}

func (e *Error) Error() string { return e.Message }

// Unauthorized builds a 401 UNAUTHORIZED error.
func Unauthorized(msg string) *Error {
	return &Error{Code: CodeUnauthorized, Message: msg, Status: http.StatusUnauthorized}
}

// NotFound builds a 404 NOT_FOUND error.
func NotFound(msg string) *Error {
	return &Error{Code: CodeNotFound, Message: msg, Status: http.StatusNotFound}
}

// Validation builds a 400 VALIDATION_ERROR error carrying optional field detail.
func Validation(msg string, fields map[string]string) *Error {
	return &Error{Code: CodeValidation, Message: msg, Status: http.StatusBadRequest, Fields: fields}
}

// Conflict builds a 409 error with a caller-supplied code (for DUPLICATE_* style codes).
func Conflict(code, msg string) *Error {
	return &Error{Code: code, Message: msg, Status: http.StatusConflict}
}

// Internal builds a 500 INTERNAL_SERVER_ERROR error.
func Internal(msg string) *Error {
	return &Error{Code: CodeInternal, Message: msg, Status: http.StatusInternalServerError}
}

// IsServerError reports whether Respond will render err as a 5xx: an error that is
// not an *Error at all, or one carrying a 5xx status, or one whose status is unset
// and therefore falls back to 500.
//
// It is the classifier a caller needs to decide whether a failure is the service's
// fault. Every 4xx in the codebase is an *Error with an explicit 4xx status, so
// this excludes the whole client-error class by construction rather than by an
// enumerated list of codes, and a new 4xx code cannot opt itself in.
func IsServerError(err error) bool {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		return true
	}
	return renderedStatus(apiErr) >= http.StatusInternalServerError
}

// renderedStatus is the status the client receives for apiErr: its own when set,
// and 500 otherwise, mirroring the fallback Respond applies to a status with no
// coherent code pairing.
func renderedStatus(apiErr *Error) int {
	if apiErr.Status <= 0 {
		return http.StatusInternalServerError
	}
	return apiErr.Status
}
