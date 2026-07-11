package apierr

import "net/http"

// Error is the single typed service error. It carries everything the wire
// contract can express, including Fields (the field-level validation detail
// that most services drop today).
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
