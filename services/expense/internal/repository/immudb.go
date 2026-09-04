package repository

import (
	"fmt"
	"log/slog"
	"strings"
)

// SnapshotIntegrityError signals a row that is missing a required snapshot
// field. The repository logs this as expense_snapshot_integrity_error
// telemetry before returning the error to the caller.
//
// It is exported so errors.As can detect it across the %w wrap boundary.
type SnapshotIntegrityError struct {
	ExpenseID     string
	MissingFields []string
}

func (e *SnapshotIntegrityError) Error() string {
	return fmt.Sprintf("expense row %s: missing required snapshot fields: %s", e.ExpenseID, strings.Join(e.MissingFields, ", "))
}

// ReportData satisfies the errkit DataCarrier interface so a report of this
// error carries the expense id and the missing field names automatically.
func (e *SnapshotIntegrityError) ReportData() map[string]any {
	return map[string]any{"expense_id": e.ExpenseID, "missing_fields": e.MissingFields}
}

// ImmudbExpenseRepository implements ExpenseRepository using immudb's SQL interface.
type ImmudbExpenseRepository struct {
	client ImmudbClient
	logger *slog.Logger
}

// NewImmudbExpenseRepository creates a new ImmudbExpenseRepository.
func NewImmudbExpenseRepository(client ImmudbClient, logger *slog.Logger) *ImmudbExpenseRepository {
	return &ImmudbExpenseRepository{
		client: client,
		logger: logger,
	}
}
