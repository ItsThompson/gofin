//go:build integration

package handler_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/auth/internal/db"
	"github.com/ItsThompson/gofin/services/auth/internal/handler"
	"github.com/ItsThompson/gofin/services/auth/internal/repository"
	"github.com/ItsThompson/gofin/services/auth/internal/service"
)

func getTestDBURL() string {
	if url := os.Getenv("TEST_DB_URL"); url != "" {
		return url
	}
	return "postgres://gofin:gofin@localhost:5432/gofin?sslmode=disable&search_path=auth"
}

func setupIntegrationTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, getTestDBURL())
	require.NoError(t, err, "failed to connect to test database")

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

func createIntegrationTestUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()

	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO auth.users (username, email, password_hash, role, currency)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text
	`,
		fmt.Sprintf("testuser_%d", time.Now().UnixNano()),
		fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()),
		"$2a$04$fakehashfakehashfakehashfakehashfakehashfakehashfakeh", // valid bcrypt format
		"user",
		"USD",
	).Scan(&userID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM auth.refresh_token_blacklist WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM auth.users WHERE id = $1::uuid`, userID)
	})

	return userID
}

// TestConcurrentRefresh_Integration verifies that when multiple concurrent
// requests attempt to refresh the same token, exactly one succeeds (200) and
// all others are rejected (401). This exercises the atomic ConsumeToken behavior
// through the full HTTP handler stack.
func TestConcurrentRefresh_Integration(t *testing.T) {
	pool := setupIntegrationTestDB(t)
	userID := createIntegrationTestUser(t, pool)

	queries := db.New(pool)
	userRepo := repository.NewPostgresUserRepository(queries)
	blacklistRepo := repository.NewPostgresBlacklistRepository(queries)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	jwtSvc := service.NewJWTService("integration-test-secret")
	pwdSvc := service.NewPasswordService(4)
	authSvc := service.NewAuthService(userRepo, blacklistRepo, jwtSvc, pwdSvc, logger)

	gin.SetMode(gin.TestMode)
	h := handler.NewRESTHandler(authSvc, logger, false, "", service.DefaultAccessTokenTTL, service.DefaultRefreshTokenTTL)
	router := gin.New()
	h.RegisterRoutes(router)

	// Generate a valid refresh token for the test user
	_, refreshToken, err := jwtSvc.GenerateTokenPair(userID, "user", "testuser")
	require.NoError(t, err)

	const concurrency = 10
	var (
		successCount atomic.Int32
		failureCount atomic.Int32
		wg           sync.WaitGroup
		start        = make(chan struct{})
	)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			<-start // All goroutines start simultaneously

			req := httptest.NewRequest("POST", "/api/auth/refresh", nil)
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{
				Name:  "gofin_refresh",
				Value: refreshToken,
			})

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			switch w.Code {
			case http.StatusOK:
				successCount.Add(1)
			case http.StatusUnauthorized:
				failureCount.Add(1)
			default:
				t.Errorf("unexpected status code: %d, body: %s", w.Code, w.Body.String())
			}
		}()
	}

	// Release all goroutines at once
	close(start)
	wg.Wait()

	assert.Equal(t, int32(1), successCount.Load(),
		"exactly one concurrent refresh should succeed")
	assert.Equal(t, int32(concurrency-1), failureCount.Load(),
		"all other concurrent refreshes should get 401")
}
