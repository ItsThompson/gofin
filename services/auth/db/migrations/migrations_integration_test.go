//go:build integration

package migrations

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdb "github.com/ItsThompson/gofin/services/auth/internal/db"
	"github.com/ItsThompson/gofin/services/dbmigrate"
)

func createDisposableAuthDatabase(t *testing.T) (string, func()) {
	t.Helper()
	baseURL := os.Getenv("TEST_DB_URL")
	if baseURL == "" {
		t.Skip("TEST_DB_URL is required for PostgreSQL integration tests")
	}

	parsed, err := url.Parse(baseURL)
	require.NoError(t, err)
	databaseName := fmt.Sprintf("gofin_report_auth_%d", time.Now().UnixNano())

	adminURL := *parsed
	adminURL.Path = "/postgres"
	adminQuery := adminURL.Query()
	adminQuery.Del("search_path")
	adminQuery.Del("x-migrations-table")
	adminURL.RawQuery = adminQuery.Encode()

	adminConfig, err := pgxpool.ParseConfig(adminURL.String())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	adminConn, err := pgx.ConnectConfig(ctx, adminConfig.ConnConfig)
	require.NoError(t, err)

	quotedName := quoteIdentifier(databaseName)
	_, err = adminConn.Exec(ctx, "CREATE DATABASE "+quotedName)
	require.NoError(t, err)

	databaseURL := *parsed
	databaseURL.Path = "/" + databaseName
	cleanup := func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = adminConn.Exec(cleanupContext, "DROP DATABASE "+quotedName+" WITH (FORCE)")
		_ = adminConn.Close(cleanupContext)
	}
	return databaseURL.String(), cleanup
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func migrationSubset(t *testing.T, names ...string) fstest.MapFS {
	t.Helper()
	migrationFS := fstest.MapFS{}
	for _, name := range names {
		contents, err := FS.ReadFile(name)
		require.NoError(t, err)
		migrationFS[name] = &fstest.MapFile{Data: contents}
	}
	return migrationFS
}

func TestOnboardingMigrationAndCompletionAreAtomicOnPostgres(t *testing.T) {
	databaseURL, cleanup := createDisposableAuthDatabase(t)
	defer cleanup()
	ctx := context.Background()

	initialMigrations := migrationSubset(t,
		"000001_create_users.up.sql",
		"000001_create_users.down.sql",
		"000002_create_refresh_token_blacklist.up.sql",
		"000002_create_refresh_token_blacklist.down.sql",
	)
	require.NoError(t, dbmigrate.RunWithFS(databaseURL, initialMigrations, "."))

	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	defer pool.Close()

	completedID := uuid.New().String()
	incompleteID := uuid.New().String()
	existingID := uuid.New().String()
	existingAt := time.Date(2025, time.March, 15, 12, 30, 0, 0, time.UTC)
	_, err = pool.Exec(ctx, `
		INSERT INTO auth.users
			(id, username, email, password_hash, role, currency, has_completed_onboarding)
		VALUES
			($1, 'completed-user', 'completed@example.com', 'hash', 'user', 'USD', true),
			($2, 'incomplete-user', 'incomplete@example.com', 'hash', 'user', 'USD', false),
			($3, 'existing-user', 'existing@example.com', 'hash', 'admin', 'EUR', true)
	`, completedID, incompleteID, existingID)
	require.NoError(t, err)

	require.NoError(t, dbmigrate.RunWithFS(databaseURL, FS, "."))
	_, err = pool.Exec(ctx, `UPDATE auth.users SET onboarding_completed_at = $1 WHERE id = $2`, existingAt, existingID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		WITH completion_time AS (SELECT now() AS completed_at)
		UPDATE auth.users AS users
		SET onboarding_completed_at = completion_time.completed_at
		FROM completion_time
		WHERE users.has_completed_onboarding
		  AND users.onboarding_completed_at IS NULL
	`)
	require.NoError(t, err)

	var completedAt, incompleteAt, preservedAt pgtype.Timestamptz
	err = pool.QueryRow(ctx, `
		SELECT
			(SELECT onboarding_completed_at FROM auth.users WHERE id = $1),
			(SELECT onboarding_completed_at FROM auth.users WHERE id = $2),
			(SELECT onboarding_completed_at FROM auth.users WHERE id = $3)
	`, completedID, incompleteID, existingID).Scan(&completedAt, &incompleteAt, &preservedAt)
	require.NoError(t, err)
	require.True(t, completedAt.Valid)
	assert.False(t, incompleteAt.Valid)
	require.True(t, preservedAt.Valid)
	assert.Equal(t, existingAt, preservedAt.Time)

	var completedCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM auth.users
		WHERE onboarding_completed_at = $1
	`, completedAt.Time).Scan(&completedCount)
	require.NoError(t, err)
	assert.Equal(t, 1, completedCount)

	for _, indexName := range []string{
		"idx_users_reporting_created_at",
		"idx_users_reporting_onboarding_completed_at",
	} {
		var exists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'auth' AND indexname = $1
			)
		`, indexName).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "missing migration index %s", indexName)
	}

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `ALTER TABLE auth.users ADD COLUMN migration_failure_marker TEXT`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE auth.users SET migration_failure_marker = 'changed' WHERE id = $1`, completedID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT 1 / 0`)
	require.Error(t, err)
	require.NoError(t, tx.Rollback(ctx))

	var markerExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'auth' AND table_name = 'users' AND column_name = 'migration_failure_marker'
		)
	`).Scan(&markerExists)
	require.NoError(t, err)
	assert.False(t, markerExists)

	concurrentID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO auth.users (id, username, email, password_hash, role, currency)
		VALUES ($1, 'concurrent-user', 'concurrent@example.com', 'hash', 'user', 'USD')
	`, concurrentID.String())
	require.NoError(t, err)

	queries := authdb.New(pool)
	const completions = 8
	results := make(chan pgtype.Timestamptz, completions)
	errors := make(chan error, completions)
	var waitGroup sync.WaitGroup
	waitGroup.Add(completions)
	for i := 0; i < completions; i++ {
		currency := []string{"USD", "EUR", "GBP", "CAD"}[i%4]
		go func() {
			defer waitGroup.Done()
			row, completionErr := queries.CompleteOnboarding(ctx, authdb.CompleteOnboardingParams{
				Currency: currency,
				ID:       pgtype.UUID{Bytes: concurrentID, Valid: true},
			})
			if completionErr != nil {
				errors <- completionErr
				return
			}
			results <- row.OnboardingCompletedAt
		}()
	}
	waitGroup.Wait()
	close(results)
	close(errors)

	for completionErr := range errors {
		require.NoError(t, completionErr)
	}
	var firstCompletion pgtype.Timestamptz
	for completionTime := range results {
		require.True(t, completionTime.Valid)
		if !firstCompletion.Valid {
			firstCompletion = completionTime
			continue
		}
		assert.Equal(t, firstCompletion.Time, completionTime.Time)
	}

	var finalCompletion pgtype.Timestamptz
	err = pool.QueryRow(ctx, `SELECT onboarding_completed_at FROM auth.users WHERE id = $1`, concurrentID.String()).Scan(&finalCompletion)
	require.NoError(t, err)
	require.True(t, finalCompletion.Valid)
	assert.Equal(t, firstCompletion.Time, finalCompletion.Time)

	down, err := FS.ReadFile("000003_add_onboarding_completed_at.down.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err)
	var columnExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'auth' AND table_name = 'users' AND column_name = 'onboarding_completed_at'
		)
	`).Scan(&columnExists)
	require.NoError(t, err)
	assert.False(t, columnExists)
}
