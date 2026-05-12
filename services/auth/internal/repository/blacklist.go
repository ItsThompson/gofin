package repository

import (
	"context"
	"time"
)

// BlacklistRepository defines the data access contract for refresh token
// blacklist operations.
type BlacklistRepository interface {
	// ConsumeToken atomically attempts to blacklist a refresh token.
	// Returns (true, nil) if the token was successfully consumed (first use).
	// Returns (false, nil) if the token was already consumed (replay).
	// Returns (false, err) on database error.
	ConsumeToken(ctx context.Context, jti, userID string, expiresAt time.Time) (bool, error)

	// BlacklistToken adds a token to the blacklist unconditionally.
	// Used by Logout where idempotency is desired (no conflict check needed).
	BlacklistToken(ctx context.Context, jti, userID string, expiresAt time.Time) error

	// CleanupExpired removes blacklist entries past their expiration.
	CleanupExpired(ctx context.Context) error

	// DeleteByUserID removes all blacklist entries for a user (data deletion).
	DeleteByUserID(ctx context.Context, userID string) error
}
