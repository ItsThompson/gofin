package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ItsThompson/gofin/services/auth/internal/db"
)

// PostgresBlacklistRepository implements BlacklistRepository using sqlc-generated queries.
type PostgresBlacklistRepository struct {
	queries *db.Queries
}

// NewPostgresBlacklistRepository creates a new PostgresBlacklistRepository.
func NewPostgresBlacklistRepository(queries *db.Queries) *PostgresBlacklistRepository {
	return &PostgresBlacklistRepository{queries: queries}
}

func (r *PostgresBlacklistRepository) BlacklistToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
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

func (r *PostgresBlacklistRepository) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	return r.queries.IsTokenBlacklisted(ctx, jti)
}

func (r *PostgresBlacklistRepository) CleanupExpired(ctx context.Context) error {
	return r.queries.CleanupExpiredBlacklist(ctx)
}

func (r *PostgresBlacklistRepository) DeleteByUserID(ctx context.Context, userID string) error {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return err
	}
	return r.queries.DeleteRefreshTokenBlacklist(ctx, uid)
}
