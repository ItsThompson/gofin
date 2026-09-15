//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/datarights/db/migrations"
	"github.com/ItsThompson/gofin/services/serverkit"
)

func datarightsIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	pool, err := serverkit.ConnectPostgres(context.Background(), databaseURL, migrations.FS)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestCountCompletedExports_IntegrationUsesStatusAndCompletionWindows(t *testing.T) {
	pool := datarightsIntegrationPool(t)
	ctx := context.Background()
	userID := "00000000-0000-0000-0000-000000000001"
	start := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	withinReport := start.AddDate(0, 0, 2)
	withinPrevious := start.AddDate(0, 0, -2)
	withinTrailing := start.AddDate(0, 0, -14)
	outsideTrailing := start.AddDate(0, 0, -29)

	_, err := pool.Exec(ctx, `DELETE FROM datarights.export_jobs WHERE user_id = $1`, userID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM datarights.export_jobs WHERE user_id = $1`, userID)
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO datarights.export_jobs (id, user_id, status, created_at, completed_at)
		VALUES
			('00000000-0000-0000-0000-000000000011', $1, 'completed', $2, $2),
			('00000000-0000-0000-0000-000000000012', $1, 'completed', $3, $3),
			('00000000-0000-0000-0000-000000000013', $1, 'completed', $4, $4),
			('00000000-0000-0000-0000-000000000014', $1, 'completed', $5, $5),
			('00000000-0000-0000-0000-000000000015', $1, 'failed', $2, $2)
	`, userID, withinReport, withinPrevious, withinTrailing, outsideTrailing)
	require.NoError(t, err)

	repo := NewPostgresJobRepository(pool)
	counts, err := repo.CountCompletedExports(ctx, ExportMetricsWindowSet{
		ReportWeekStart:        start,
		ReportWeekEnd:          start.AddDate(0, 0, 7),
		PreviousWeekStart:      start.AddDate(0, 0, -7),
		PreviousWeekEnd:        start,
		TrailingFourWeeksStart: start.AddDate(0, 0, -28),
		TrailingFourWeeksEnd:   start,
	})
	require.NoError(t, err)
	assert.Equal(t, CompletedExportCounts{ReportWeek: 1, PreviousWeek: 1, TrailingFourWeeksTotal: 3}, counts)

	var exists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'datarights' AND indexname = 'idx_export_jobs_completed_at'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "missing completed export reporting index")
}
