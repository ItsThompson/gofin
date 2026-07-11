package deletion

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFuncProvider_Name(t *testing.T) {
	p := NewFuncProvider("auth", func(context.Context, string) error { return nil })
	assert.Equal(t, "auth", p.Name())
}

func TestNewFuncProvider_Delete_DelegatesArgsAndSuccess(t *testing.T) {
	var gotUser string
	p := NewFuncProvider("finance", func(_ context.Context, userID string) error {
		gotUser = userID
		return nil
	})

	err := p.Delete(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "user-123", gotUser)
}

func TestNewFuncProvider_Delete_PropagatesError(t *testing.T) {
	wantErr := errors.New("service unavailable")
	p := NewFuncProvider("expense", func(context.Context, string) error { return wantErr })

	err := p.Delete(context.Background(), "user-abc")

	require.ErrorIs(t, err, wantErr)
}

func TestNewFuncProvider_Delete_Idempotent_RepeatedCallsSafe(t *testing.T) {
	// A provider whose data is already gone is a no-op returning nil. Calling
	// Delete repeatedly for the same user must stay nil (Provider idempotency).
	calls := 0
	p := NewFuncProvider("auth", func(context.Context, string) error {
		calls++
		return nil
	})

	for range 3 {
		require.NoError(t, p.Delete(context.Background(), "user-1"))
	}
	assert.Equal(t, 3, calls)
}

func TestNewFuncProvider_RegistrationOrderPreserved(t *testing.T) {
	// Registration order is the execution order; auth must be able to run last.
	registry := NewRegistry()
	registry.Register(NewFuncProvider("finance", func(context.Context, string) error { return nil }))
	registry.Register(NewFuncProvider("expense", func(context.Context, string) error { return nil }))
	registry.Register(NewFuncProvider("auth", func(context.Context, string) error { return nil }))

	all := registry.All()
	require.Len(t, all, 3)
	assert.Equal(t, []string{"finance", "expense", "auth"},
		[]string{all[0].Name(), all[1].Name(), all[2].Name()})
}
