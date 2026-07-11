//go:build !docker

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestInMemoryClient() *inMemoryImmudbClient {
	return &inMemoryImmudbClient{rows: make([]map[string]interface{}, 0)}
}

// TestInMemoryClient_UpdateReturnsExplicitError locks in C3: the local stub no
// longer silently no-ops an UPDATE (which would leave a corrected expense's
// original row active and double-count); it fails loudly instead.
func TestInMemoryClient_UpdateReturnsExplicitError(t *testing.T) {
	client := newTestInMemoryClient()

	result, err := client.SQLExec(context.Background(),
		"UPDATE expenses SET status = 'corrected' WHERE id = @id;",
		map[string]interface{}{"id": "exp-1"})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported in the local in-memory immudb stub")
}
