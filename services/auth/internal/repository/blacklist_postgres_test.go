package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/auth/internal/db"
)

// fakeDBTX implements db.DBTX for unit testing the repository layer.
type fakeDBTX struct {
	execFn     func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (f *fakeDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if f.queryFn != nil {
		return f.queryFn(ctx, sql, args...)
	}
	return nil, nil
}

func (f *fakeDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if f.queryRowFn != nil {
		return f.queryRowFn(ctx, sql, args...)
	}
	return &fakeRow{err: pgx.ErrNoRows}
}

// fakeRow implements pgx.Row for test assertions.
type fakeRow struct {
	jti string
	err error
}

func (r *fakeRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		if p, ok := dest[0].(*string); ok {
			*p = r.jti
		}
	}
	return nil
}

func TestConsumeToken_Success(t *testing.T) {
	fake := &fakeDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			// Simulate successful INSERT: returns the jti
			return &fakeRow{jti: "test-jti-123"}
		},
	}
	queries := db.New(fake)
	repo := NewPostgresBlacklistRepository(queries)

	consumed, err := repo.ConsumeToken(
		context.Background(),
		"test-jti-123",
		"550e8400-e29b-41d4-a716-446655440000",
		time.Now().Add(24*time.Hour),
	)

	require.NoError(t, err)
	assert.True(t, consumed, "first consume should return true")
}

func TestConsumeToken_AlreadyConsumed(t *testing.T) {
	fake := &fakeDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			// Simulate ON CONFLICT DO NOTHING: no row returned
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	queries := db.New(fake)
	repo := NewPostgresBlacklistRepository(queries)

	consumed, err := repo.ConsumeToken(
		context.Background(),
		"test-jti-123",
		"550e8400-e29b-41d4-a716-446655440000",
		time.Now().Add(24*time.Hour),
	)

	require.NoError(t, err)
	assert.False(t, consumed, "second consume should return false (already consumed)")
}

func TestConsumeToken_DBError(t *testing.T) {
	dbErr := errors.New("connection refused")
	fake := &fakeDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &fakeRow{err: dbErr}
		},
	}
	queries := db.New(fake)
	repo := NewPostgresBlacklistRepository(queries)

	consumed, err := repo.ConsumeToken(
		context.Background(),
		"test-jti-123",
		"550e8400-e29b-41d4-a716-446655440000",
		time.Now().Add(24*time.Hour),
	)

	assert.False(t, consumed)
	require.Error(t, err)
	assert.Equal(t, dbErr, err)
}

func TestConsumeToken_InvalidUserID(t *testing.T) {
	fake := &fakeDBTX{}
	queries := db.New(fake)
	repo := NewPostgresBlacklistRepository(queries)

	consumed, err := repo.ConsumeToken(
		context.Background(),
		"test-jti-123",
		"not-a-valid-uuid",
		time.Now().Add(24*time.Hour),
	)

	assert.False(t, consumed)
	require.Error(t, err)
}

func TestConsumeToken_AfterBlacklistToken(t *testing.T) {
	// Simulates the "logout then refresh" scenario:
	// BlacklistToken is called first (logout), then ConsumeToken is called (refresh attempt).
	// Since the token is already in the blacklist, ConsumeToken should return false.
	callCount := 0
	fake := &fakeDBTX{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
			// BlacklistToken uses Exec (no RETURNING)
			return pgconn.CommandTag{}, nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			callCount++
			// After BlacklistToken inserted the row, ConsumeToken's INSERT conflicts
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	queries := db.New(fake)
	repo := NewPostgresBlacklistRepository(queries)
	ctx := context.Background()
	userID := "550e8400-e29b-41d4-a716-446655440000"
	jti := "test-jti-456"
	expiresAt := time.Now().Add(24 * time.Hour)

	// Step 1: Logout blacklists the token
	err := repo.BlacklistToken(ctx, jti, userID, expiresAt)
	require.NoError(t, err)

	// Step 2: Refresh attempt tries to consume the same token
	consumed, err := repo.ConsumeToken(ctx, jti, userID, expiresAt)
	require.NoError(t, err)
	assert.False(t, consumed, "ConsumeToken after BlacklistToken should return false")
}
