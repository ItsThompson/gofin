package service

import (
	"context"
	"fmt"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
)

// UserEmailResolver resolves a user's email address from their user ID.
type UserEmailResolver interface {
	ResolveEmail(ctx context.Context, userID string) (string, error)
}

// AuthUserEmailResolver resolves user email via the auth gRPC service.
type AuthUserEmailResolver struct {
	authClient authpb.AuthServiceClient
}

// NewAuthUserEmailResolver creates a resolver backed by the auth gRPC client.
func NewAuthUserEmailResolver(authClient authpb.AuthServiceClient) *AuthUserEmailResolver {
	return &AuthUserEmailResolver{authClient: authClient}
}

// ResolveEmail fetches the user's email from the auth service.
func (r *AuthUserEmailResolver) ResolveEmail(ctx context.Context, userID string) (string, error) {
	resp, err := r.authClient.GetUser(ctx, &authpb.GetUserRequest{UserId: userID})
	if err != nil {
		return "", fmt.Errorf("resolving user email: %w", err)
	}

	email := resp.GetEmail()
	if email == "" {
		return "", fmt.Errorf("user %s has no email address", userID)
	}

	return email, nil
}
