package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpenseDeletionProvider_Name(t *testing.T) {
	p := NewExpenseDeletionProvider(nil)
	assert.Equal(t, "expense", p.Name())
}

func TestExpenseDeletionProvider_Delete_Success(t *testing.T) {
	mock := &mockExpenseClient{}
	p := NewExpenseDeletionProvider(mock)

	err := p.Delete(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "user-123", mock.anonymizeCalledWith)
}

func TestExpenseDeletionProvider_Delete_Error(t *testing.T) {
	mock := &mockExpenseClient{
		anonymizeErr: fmt.Errorf("service unavailable"),
	}
	p := NewExpenseDeletionProvider(mock)

	err := p.Delete(context.Background(), "user-789")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "service unavailable")
	assert.Equal(t, "user-789", mock.anonymizeCalledWith)
}
