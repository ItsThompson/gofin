package providers

import (
	"context"
	"fmt"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
)

// Compile-time check that ProfileProvider implements DataProvider.
var _ engine.DataProvider = (*ProfileProvider)(nil)

// ProfileProvider fetches user profile data from the auth service.
type ProfileProvider struct {
	authClient authpb.AuthServiceClient
}

// NewProfileProvider creates a ProfileProvider backed by the auth gRPC client.
func NewProfileProvider(authClient authpb.AuthServiceClient) *ProfileProvider {
	return &ProfileProvider{authClient: authClient}
}

// Name returns the CSV filename for this provider.
func (p *ProfileProvider) Name() string {
	return "profile"
}

// Headers returns the CSV column headers for profile data.
func (p *ProfileProvider) Headers() []string {
	return []string{"username", "email", "currency", "role", "account_created_at"}
}

// Collect fetches the user's profile from the auth service and returns a single row.
func (p *ProfileProvider) Collect(ctx context.Context, userID string) ([][]string, error) {
	resp, err := p.authClient.GetUser(ctx, &authpb.GetUserRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("fetching user profile: %w", err)
	}

	row := []string{
		resp.GetUsername(),
		resp.GetEmail(),
		resp.GetCurrency(),
		resp.GetRole(),
		resp.GetCreatedAt(),
	}

	return [][]string{row}, nil
}
