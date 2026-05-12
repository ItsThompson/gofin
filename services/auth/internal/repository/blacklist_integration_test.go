//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/auth/internal/db"
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
)

func getTestDBURL() string {
	if url := os.Getenv("TEST_DB_URL"); url != "" {
		return url
	}
	return "postgres://gofin:gofin@localhost:5432/gofin?sslmode=disable&search_path=auth"
}

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, getTestDBURL())
	require.NoError(t, err, "failed to connect to test database")

	// Ensure the schema and tables exist
	_, err = pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS auth`)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS auth.users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(50) NOT NULL UNIQUE,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(10) NOT NULL DEFAULT 'user',
			currency VARCHAR(3) NOT NULL DEFAULT 'USD',
			has_completed_onboarding BOOLEAN NOT NULL DEFAULT false,
			tokens_revoked_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS auth.refresh_token_blacklist (
			jti VARCHAR(36) PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES auth.users(id),
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func createTestUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()

	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO auth.users (username, email, password_hash, role, currency)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text
	`, fmt.Sprintf("testuser_%d", time.Now().UnixNano()), fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()), "fakehash", "user", "USD").Scan(&userID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM auth.refresh_token_blacklist WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM auth.users WHERE id = $1::uuid`, userID)
	})

	return userID
}

func TestConsumeToken_Concurrent_Integration(t *testing.T) {
	pool := setupTestDB(t)
	userID := createTestUser(t, pool)

	queries := db.New(pool)
	repo := repository.NewPostgresBlacklistRepository(queries)

	ctx := context.Background()
	jti := fmt.Sprintf("test-jti-%d", time.Now().UnixNano())
	expiresAt := time.Now().Add(24 * time.Hour)

	const goroutines = 10
	var successCount atomic.Int32
	var wg sync.WaitGroup

	// All goroutines start simultaneously
	start := make(chan struct{})

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start // Wait for the signal

			consumed, err := repo.ConsumeToken(ctx, jti, userID, expiresAt)
			require.NoError(t, err)
			if consumed {
				successCount.Add(1)
			}
		}()
	}

	// Release all goroutines at once
	close(start)
	wg.Wait()

	assert.Equal(t, int32(1), successCount.Load(),
		"exactly one goroutine should successfully consume the token")
}

func TestConsumeToken_AfterBlacklistToken_Integration(t *testing.T) {
	pool := setupTestDB(t)
	userID := createTestUser(t, pool)

	queries := db.New(pool)
	repo := repository.NewPostgresBlacklistRepository(queries)

	ctx := context.Background()
	jti := fmt.Sprintf("test-jti-logout-%d", time.Now().UnixNano())
	expiresAt := time.Now().Add(24 * time.Hour)

	// Step 1: Logout blacklists the token
	err := repo.BlacklistToken(ctx, jti, userID, expiresAt)
	require.NoError(t, err)

	// Step 2: Refresh attempt tries to consume the already-blacklisted token
	consumed, err := repo.ConsumeToken(ctx, jti, userID, expiresAt)
	require.NoError(t, err)
	assert.False(t, consumed, "ConsumeToken after BlacklistToken (logout) should return false")
}

func TestConsumeToken_DoubleConsume_Integration(t *testing.T) {
	pool := setupTestDB(t)
	userID := createTestUser(t, pool)

	queries := db.New(pool)
	repo := repository.NewPostgresBlacklistRepository(queries)

	ctx := context.Background()
	jti := fmt.Sprintf("test-jti-double-%d", time.Now().UnixNano())
	expiresAt := time.Now().Add(24 * time.Hour)

	// First consume succeeds
	consumed, err := repo.ConsumeToken(ctx, jti, userID, expiresAt)
	require.NoError(t, err)
	assert.True(t, consumed, "first ConsumeToken should succeed")

	// Second consume fails (already consumed)
	consumed, err = repo.ConsumeToken(ctx, jti, userID, expiresAt)
	require.NoError(t, err)
	assert.False(t, consumed, "second ConsumeToken should fail (already consumed)")
}

func TestBlacklistToken_Idempotent_Integration(t *testing.T) {
	pool := setupTestDB(t)
	userID := createTestUser(t, pool)

	queries := db.New(pool)
	repo := repository.NewPostgresBlacklistRepository(queries)

	ctx := context.Background()
	jti := fmt.Sprintf("test-jti-idempotent-%d", time.Now().UnixNano())
	expiresAt := time.Now().Add(24 * time.Hour)

	// First blacklist succeeds
	err := repo.BlacklistToken(ctx, jti, userID, expiresAt)
	require.NoError(t, err)

	// Second blacklist also succeeds (idempotent via ON CONFLICT DO NOTHING)
	err = repo.BlacklistToken(ctx, jti, userID, expiresAt)
	require.NoError(t, err, "BlacklistToken should be idempotent")
}
