package access

import "context"

// TokenValidationResult holds the identity returned by the auth service after a
// successful token validation. Its canonical home is this package because it is
// the value consumed by AccessControl.
type TokenValidationResult struct {
	UserID    string
	Role      string
	Username  string
	AssumedBy string
}

// TokenValidator abstracts the gRPC call to the auth service's ValidateToken
// RPC. Its canonical home is this package because AccessControl consumes it.
// The concrete gRPC client is unchanged and satisfies this interface
// structurally. Defining the interface here (rather than in middleware) keeps
// the access model self-contained and lets AccessControl be unit-tested with a
// fake validator, no gRPC connection required.
type TokenValidator interface {
	ValidateToken(ctx context.Context, accessToken string) (*TokenValidationResult, error)
}
