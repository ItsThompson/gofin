package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/gateway/internal/access"
)

// validateTokenClient is the narrow slice of authpb.AuthServiceClient that
// GRPCTokenValidator depends on. Depending on the one RPC it calls (rather than
// the full client) keeps the validator testable with a fake that implements
// only ValidateToken; authpb.AuthServiceClient satisfies it structurally.
type validateTokenClient interface {
	ValidateToken(ctx context.Context, in *authpb.ValidateTokenRequest, opts ...grpc.CallOption) (*authpb.ValidateTokenResponse, error)
}

// GRPCTokenValidator implements access.TokenValidator by calling the
// auth service's ValidateToken gRPC endpoint.
type GRPCTokenValidator struct {
	client  validateTokenClient
	timeout time.Duration
}

// NewGRPCTokenValidator creates a validator backed by the given gRPC connection.
// timeout bounds every ValidateToken call so a hung auth service returns a fast
// deadline error instead of blocking the gateway worker indefinitely.
func NewGRPCTokenValidator(conn *grpc.ClientConn, timeout time.Duration) *GRPCTokenValidator {
	return &GRPCTokenValidator{
		client:  authpb.NewAuthServiceClient(conn),
		timeout: timeout,
	}
}

// ValidateToken calls the auth service to validate an access token.
// Returns the user identity on success or an error on failure.
//
// The call is bounded by a per-call timeout derived from the incoming request
// context, so a client disconnect still cancels it immediately while a hung
// auth service is capped at v.timeout. A timeout surfaces as an error that
// unwraps to context.DeadlineExceeded (and carries gRPC codes.DeadlineExceeded),
// which AccessControl maps to 503 rather than 401.
func (v *GRPCTokenValidator) ValidateToken(ctx context.Context, accessToken string) (*access.TokenValidationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

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
