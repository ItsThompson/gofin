package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
)

const completedExportCountQuery = `
	SELECT
		COUNT(*) FILTER (WHERE completed_at >= $2 AND completed_at < $3),
		COUNT(*) FILTER (WHERE completed_at >= $4 AND completed_at < $5),
		COUNT(*) FILTER (WHERE completed_at >= $6 AND completed_at < $7)
	FROM datarights.export_jobs
	WHERE status = $1
	  AND completed_at >= $6
	  AND completed_at < $3`

// CountCompletedExports returns all completed-export counts in one database
// operation. Each range is half-open: [start, end).
func (r *PostgresJobRepository) CountCompletedExports(ctx context.Context, windows ExportMetricsWindowSet) (CompletedExportCounts, error) {
	return countCompletedExports(ctx, r.pool, windows)
}

// queryRower keeps the count operation testable without a database connection.
type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func countCompletedExports(ctx context.Context, db queryRower, windows ExportMetricsWindowSet) (CompletedExportCounts, error) {
	var counts CompletedExportCounts
	if err := db.QueryRow(ctx, completedExportCountQuery,
		model.StatusCompleted,
		windows.ReportWeekStart,
		windows.ReportWeekEnd,
		windows.PreviousWeekStart,
		windows.PreviousWeekEnd,
		windows.TrailingFourWeeksStart,
		windows.TrailingFourWeeksEnd,
	).Scan(&counts.ReportWeek, &counts.PreviousWeek, &counts.TrailingFourWeeksTotal); err != nil {
		return CompletedExportCounts{}, fmt.Errorf("counting completed exports: %w", err)
	}
	return counts, nil
}

var _ ExportMetricsRepository = (*PostgresJobRepository)(nil)
