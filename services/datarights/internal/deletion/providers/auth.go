package providers

import (
	"context"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/internal/deletion"
)

// Compile-time check that AuthDeletionProvider implements DeletionProvider.
var _ deletion.DeletionProvider = (*AuthDeletionProvider)(nil)

// AuthDeletionProvider deletes all auth-related data for a user via gRPC.
// Must be registered last: user cannot authenticate after this provider runs.
type AuthDeletionProvider struct {
	authClient authpb.AuthServiceClient
}

// NewAuthDeletionProvider creates an AuthDeletionProvider backed by the auth gRPC client.
func NewAuthDeletionProvider(authClient authpb.AuthServiceClient) *AuthDeletionProvider {
	return &AuthDeletionProvider{authClient: authClient}
}

// Name returns a human-readable identifier for this provider.
func (p *AuthDeletionProvider) Name() string {
	return "auth"
}

// Delete removes all auth data for the given user.
func (p *AuthDeletionProvider) Delete(ctx context.Context, userID string) error {
	_, err := p.authClient.DeleteUserData(ctx, &authpb.DeleteUserDataRequest{
		UserId: userID,
	})
	return err
}
