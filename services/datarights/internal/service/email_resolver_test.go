package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmailResolver implements UserEmailResolver for testing.
type mockEmailResolver struct {
	emails map[string]string
	err    error
}

func (m *mockEmailResolver) ResolveEmail(_ context.Context, userID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	email, ok := m.emails[userID]
	if !ok {
		return "", fmt.Errorf("user %s not found", userID)
	}
	return email, nil
}

func TestAuthUserEmailResolver_Interface(t *testing.T) {
	// Verify mockEmailResolver satisfies UserEmailResolver
	var _ UserEmailResolver = (*mockEmailResolver)(nil)
}

func TestResolveUserEmail_WithResolver(t *testing.T) {
	resolver := &mockEmailResolver{
		emails: map[string]string{"user-123": "alex@example.com"},
	}

	svc := NewExportService(nil, nil, WithEmailResolver(resolver))
	email, err := svc.resolveUserEmail(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "alex@example.com", email)
}

func TestResolveUserEmail_NoResolver(t *testing.T) {
	svc := NewExportService(nil, nil)
	email, err := svc.resolveUserEmail(context.Background(), "user-123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no email resolver configured")
	assert.Equal(t, "", email)
}

func TestResolveUserEmail_ResolverError(t *testing.T) {
	resolver := &mockEmailResolver{
		err: fmt.Errorf("auth service unavailable"),
	}

	svc := NewExportService(nil, nil, WithEmailResolver(resolver))
	email, err := svc.resolveUserEmail(context.Background(), "user-123")

	assert.Error(t, err)
	assert.Equal(t, "", email)
}
