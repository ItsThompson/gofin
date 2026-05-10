package repository

import (
	"context"
	"time"
)

// BlacklistRepository defines the data access contract for refresh token blacklist operations.
type BlacklistRepository interface {
	BlacklistToken(ctx context.Context, jti, userID string, expiresAt time.Time) error
	IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)
	CleanupExpired(ctx context.Context) error
	DeleteByUserID(ctx context.Context, userID string) error
}
