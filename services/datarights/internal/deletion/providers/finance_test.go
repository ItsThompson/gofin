package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinanceDeletionProvider_Name(t *testing.T) {
	p := NewFinanceDeletionProvider(nil)
	assert.Equal(t, "finance", p.Name())
}

func TestFinanceDeletionProvider_Delete_Success(t *testing.T) {
	mock := &mockFinanceClient{}
	p := NewFinanceDeletionProvider(mock)

	err := p.Delete(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "user-123", mock.deleteCalledWith)
}

func TestFinanceDeletionProvider_Delete_Error(t *testing.T) {
	mock := &mockFinanceClient{
		deleteAllUserDataErr: fmt.Errorf("connection refused"),
	}
	p := NewFinanceDeletionProvider(mock)

	err := p.Delete(context.Background(), "user-456")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
	assert.Equal(t, "user-456", mock.deleteCalledWith)
}
