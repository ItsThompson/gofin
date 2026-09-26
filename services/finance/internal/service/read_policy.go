package service

import "context"

type readPolicyContextKey struct{}

// WithCacheBypass marks a request as an explicit forced refresh. The marker
// stays on the request context so all dependent reads use the same policy.
func WithCacheBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, readPolicyContextKey{}, true)
}

func cacheBypass(ctx context.Context) bool {
	bypass, _ := ctx.Value(readPolicyContextKey{}).(bool)
	return bypass
}
