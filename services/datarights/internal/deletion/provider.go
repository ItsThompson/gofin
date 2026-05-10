package deletion

import "context"

// DeletionProvider deletes one category of user data.
// Each provider is responsible for all data in its domain.
// Implementations must be idempotent: calling Delete for a user
// whose data is already gone is a no-op (returns nil).
type DeletionProvider interface {
	// Name returns a human-readable identifier for this provider.
	// Used in logging, metrics labels, and error messages.
	Name() string

	// Delete removes or anonymizes all data for the given user.
	// Must be idempotent: repeated calls for the same user are safe.
	// Returns nil on success (including "nothing to delete").
	Delete(ctx context.Context, userID string) error
}
