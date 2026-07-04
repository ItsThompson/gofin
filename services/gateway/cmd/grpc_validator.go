package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/gateway/internal/access"
)

// GRPCTokenValidator implements access.TokenValidator by calling the
// auth service's ValidateToken gRPC endpoint.
type GRPCTokenValidator struct {
	client authpb.AuthServiceClient
}

// NewGRPCTokenValidator creates a validator backed by the given gRPC connection.
func NewGRPCTokenValidator(conn *grpc.ClientConn) *GRPCTokenValidator {
	return &GRPCTokenValidator{
		client: authpb.NewAuthServiceClient(conn),
	}
}

// ValidateToken calls the auth service to validate an access token.
// Returns the user identity on success or an error on failure.
func (v *GRPCTokenValidator) ValidateToken(ctx context.Context, accessToken string) (*access.TokenValidationResult, error) {
	resp, err := v.client.ValidateToken(ctx, &authpb.ValidateTokenRequest{
		AccessToken: accessToken,
	})
	if err != nil {
		return nil, fmt.Errorf("auth service validation failed: %w", err)
	}

	return &access.TokenValidationResult{
		UserID:    resp.GetUserId(),
		Role:      resp.GetRole(),
		Username:  resp.GetUsername(),
		AssumedBy: resp.GetAssumedBy(),
	}, nil
}
