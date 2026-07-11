package deletion

import "context"

// funcProvider adapts a name and a deletion function into a Provider, so a
// caller can register a provider without declaring a struct per data domain.
type funcProvider struct {
	name string
	fn   func(ctx context.Context, userID string) error
}

// NewFuncProvider builds a Provider from a name and a deletion function. It
// collapses the near-identical per-domain provider structs (auth, expense,
// finance) that each wrapped a single gRPC delete call. The function must be
// idempotent (see Provider): repeated calls for a user whose data is already
// gone must return nil.
func NewFuncProvider(name string, fn func(ctx context.Context, userID string) error) Provider {
	return &funcProvider{name: name, fn: fn}
}

// Name returns the provider's human-readable identifier.
func (p *funcProvider) Name() string { return p.name }

// Delete runs the wrapped deletion function for the given user.
func (p *funcProvider) Delete(ctx context.Context, userID string) error {
	return p.fn(ctx, userID)
}
