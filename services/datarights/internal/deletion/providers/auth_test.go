package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthDeletionProvider_Name(t *testing.T) {
	p := NewAuthDeletionProvider(nil)
	assert.Equal(t, "auth", p.Name())
}

func TestAuthDeletionProvider_Delete_Success(t *testing.T) {
	mock := &mockAuthClient{}
	p := NewAuthDeletionProvider(mock)

	err := p.Delete(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "user-123", mock.deleteUserDataCalledWith)
}

func TestAuthDeletionProvider_Delete_Error(t *testing.T) {
	mock := &mockAuthClient{
		deleteUserDataErr: fmt.Errorf("deadline exceeded"),
	}
	p := NewAuthDeletionProvider(mock)

	err := p.Delete(context.Background(), "user-abc")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
	assert.Equal(t, "user-abc", mock.deleteUserDataCalledWith)
}
