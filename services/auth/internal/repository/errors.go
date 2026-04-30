package repository

import "fmt"

// DuplicateError is returned when a unique constraint is violated on INSERT.
// Constraint carries the PostgreSQL constraint name (e.g., "users_email_key",
// "users_username_key") so the caller can map to a specific error code.
type DuplicateError struct {
	Constraint string
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("duplicate value violates constraint: %s", e.Constraint)
}
