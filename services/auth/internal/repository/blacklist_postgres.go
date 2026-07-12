package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ItsThompson/gofin/services/auth/internal/db"
	"github.com/ItsThompson/gofin/services/pgutil"
)

// PostgresBlacklistRepository implements BlacklistRepository using sqlc-generated queries.
type PostgresBlacklistRepository struct {
	queries *db.Queries
}

// NewPostgresBlacklistRepository creates a new PostgresBlacklistRepository.
func NewPostgresBlacklistRepository(queries *db.Queries) *PostgresBlacklistRepository {
	return &PostgresBlacklistRepository{queries: queries}
}

func (r *PostgresBlacklistRepository) ConsumeToken(ctx context.Context, jti, userID string, expiresAt time.Time) (bool, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return false, err
	}

	// ConsumeRefreshToken returns the jti if INSERT succeeded,
	// or pgx.ErrNoRows if ON CONFLICT triggered (already consumed).
	_, err = r.queries.ConsumeRefreshToken(ctx, db.ConsumeRefreshTokenParams{
		Jti:    jti,
		UserID: uid,
		ExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
	})
	if err != nil {
		if pgutil.IsNoRows(err) {
			return false, nil // Already consumed: not an error, just a signal
		}
		return false, err // Actual DB error
	}
	return true, nil // Successfully consumed
}

func (r *PostgresBlacklistRepository) BlacklistToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}

	return r.queries.BlacklistToken(ctx, db.BlacklistTokenParams{
		Jti:    jti,
		UserID: uid,
		ExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
	})
}

func (r *PostgresBlacklistRepository) CleanupExpired(ctx context.Context) error {
	return r.queries.CleanupExpiredBlacklist(ctx)
}

func (r *PostgresBlacklistRepository) DeleteByUserID(ctx context.Context, userID string) error {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.DeleteRefreshTokenBlacklist(ctx, uid)
}
