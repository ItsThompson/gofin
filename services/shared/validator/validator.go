// Package validator accumulates field-level request validation errors. It
// records the first error per field so a caller can collect every violation
// in one pass and render them as the fields detail of a validation error.
package validator

// RequestValidator accumulates field-level validation errors, keeping the
// first error recorded per field.
type RequestValidator struct {
	fields map[string]string
}

// New returns an empty RequestValidator.
func New() *RequestValidator {
	return &RequestValidator{fields: make(map[string]string)}
}

// Check records msg for field when cond is false. The first error per field
// wins: later checks for the same field are ignored.
func (v *RequestValidator) Check(cond bool, field, msg string) {
	if cond {
		return
	}
	if _, ok := v.fields[field]; !ok {
		v.fields[field] = msg
	}
}

// HasErrors reports whether any field failed a check.
func (v *RequestValidator) HasErrors() bool {
	return len(v.fields) > 0
}

// Errors returns the accumulated field errors.
func (v *RequestValidator) Errors() map[string]string {
	return v.fields
}
