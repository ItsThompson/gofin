//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ItsThompson/gofin/services/dbmigrate"
	"github.com/ItsThompson/gofin/services/finance/db/migrations"
	"github.com/ItsThompson/gofin/services/finance/internal/db"
	"github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

func financeIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	require.NoError(t, dbmigrate.RunWithFS(databaseURL, migrations.FS, "."))
	pool, err := pgxpool.New(context.Background(), databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func financeIntegrationWindows(start time.Time) *reportingpb.ReportWindowSet {
	return &reportingpb.ReportWindowSet{
		RequestedReportWeekStart: timestamppb.New(start),
		ReportWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(start), End: timestamppb.New(start.AddDate(0, 0, 7)),
		},
		PreviousWeek: &reportingpb.UtcWindow{
			Start: timestamppb.New(start.AddDate(0, 0, -7)), End: timestamppb.New(start),
		},
		TrailingFourWeeks: &reportingpb.UtcWindow{
			Start: timestamppb.New(start.AddDate(0, 0, -28)), End: timestamppb.New(start),
		},
	}
}

func TestProductUsageRepository_IntegrationCountsBoundariesAndStatuses(t *testing.T) {
	pool := financeIntegrationPool(t)
	ctx := context.Background()
	userID := uuid.New()
	tagID := uuid.New()
	start := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	withinPrevious := start.AddDate(0, 0, -2)
	withinTrailing := start.AddDate(0, 0, -14)
	outsideTrailing := start.AddDate(0, 0, -29)
	withinReport := start.AddDate(0, 0, 2)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM finance.pro_rata_schedules WHERE user_id = $1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM finance.tags WHERE user_id = $1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM finance.budget_periods WHERE user_id = $1`, userID)
	})

	_, err := pool.Exec(ctx, `
		INSERT INTO finance.budget_periods
			(id, user_id, year, month, budget_amount, essentials_percent, desires_percent, savings_percent, created_at)
		VALUES
			($1, $2, 2026, 1, 1000, 50, 30, 20, $3),
			($4, $2, 2025, 12, 1000, 50, 30, 20, $5),
			($6, $2, 2025, 11, 1000, 50, 30, 20, $7),
			($8, $2, 2025, 10, 1000, 50, 30, 20, $9)
	`, uuid.New(), userID, withinReport, uuid.New(), withinPrevious, uuid.New(), withinTrailing, uuid.New(), outsideTrailing)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO finance.tags (id, user_id, name, is_default, created_at)
		VALUES
			($1, $2, 'default', true, $3),
			($4, $2, 'custom', false, $5),
			($6, $2, 'outside', false, $7)
	`, tagID, userID, withinReport, uuid.New(), withinPrevious, uuid.New(), outsideTrailing)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO finance.pro_rata_schedules
			(id, user_id, pro_rata_group, name, amount, currency, expense_type, tag_id, target_year, target_month, installment_index, installment_total, status, created_at)
		VALUES
			($1, $2, $3, 'report-pending', 100, 'USD', 'essentials', $4, 2026, 1, 1, 1, 'pending', $5),
			($6, $2, $3, 'previous-applied', 100, 'USD', 'desires', $4, 2025, 12, 1, 1, 'applied', $7),
			($8, $2, $3, 'trailing-pending', 100, 'USD', 'savings', $4, 2025, 11, 1, 1, 'pending', $9),
			($10, $2, $3, 'outside-applied', 100, 'USD', 'savings', $4, 2025, 10, 1, 1, 'applied', $11)
	`, uuid.New(), userID, uuid.New(), tagID, withinReport, uuid.New(), withinPrevious, uuid.New(), withinTrailing, uuid.New(), outsideTrailing)
	require.NoError(t, err)

	repo := NewPostgresFinanceRepository(db.New(pool))
	windows := financeIntegrationWindows(start)
	budgetPeriods, err := repo.CountBudgetPeriods(ctx, windows)
	require.NoError(t, err)
	assert.Equal(t, ProductUsageCounts{ReportWeek: 1, PreviousWeek: 1, TrailingFourWeeksTotal: 3}, budgetPeriods)

	tags, err := repo.CountTags(ctx, windows)
	require.NoError(t, err)
	assert.Equal(t, ProductUsageCounts{ReportWeek: 1, PreviousWeek: 1, TrailingFourWeeksTotal: 2}, tags)

	schedules, err := repo.CountProRataSchedules(ctx, windows)
	require.NoError(t, err)
	assert.Equal(t, ProductUsageCounts{ReportWeek: 1, PreviousWeek: 1, TrailingFourWeeksTotal: 3}, schedules)

	for _, indexName := range []string{
		"idx_budget_periods_created_at",
		"idx_tags_created_at",
		"idx_pro_rata_schedules_created_at",
	} {
		var exists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'finance' AND indexname = $1
			)
		`, indexName).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "missing reporting index %s", indexName)
	}
}
