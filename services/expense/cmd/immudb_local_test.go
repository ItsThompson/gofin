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

// TestInMemoryClient_UpdateReturnsExplicitError asserts the local stub fails
// loudly on UPDATE rather than silently no-opping, which would leave a
// corrected expense's original row active and double-count.
func TestInMemoryClient_UpdateReturnsExplicitError(t *testing.T) {
	client := newTestInMemoryClient()

	result, err := client.SQLExec(context.Background(),
		"UPDATE expenses SET status = 'corrected' WHERE id = @id;",
		map[string]interface{}{"id": "exp-1"})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported in the local in-memory immudb stub")
}

// TestInMemoryClient_SelectByIDIsUserScoped asserts select-by-id is scoped to
// the requesting user: the stub returns no row under an empty user scope and
// never returns another user's row.
func TestInMemoryClient_SelectByIDIsUserScoped(t *testing.T) {
	ctx := context.Background()
	client := newTestInMemoryClient()

	_, err := client.SQLExec(ctx,
		"INSERT INTO expenses (id, user_id, status) VALUES (@id, @user_id, @status);",
		map[string]interface{}{"id": "exp-1", "user_id": "user-a", "status": "active"})
	require.NoError(t, err)

	const selectByID = "SELECT id, user_id, status FROM expenses WHERE id = @id AND user_id = @user_id;"

	// Empty user scope must return no row.
	emptyScope, err := client.SQLQuery(ctx, selectByID,
		map[string]interface{}{"id": "exp-1", "user_id": ""})
	require.NoError(t, err)
	assert.Empty(t, emptyScope.Rows)

	// A different user cannot read user-a's row.
	otherUser, err := client.SQLQuery(ctx, selectByID,
		map[string]interface{}{"id": "exp-1", "user_id": "user-b"})
	require.NoError(t, err)
	assert.Empty(t, otherUser.Rows)

	// The owner still reads their own row.
	owner, err := client.SQLQuery(ctx, selectByID,
		map[string]interface{}{"id": "exp-1", "user_id": "user-a"})
	require.NoError(t, err)
	require.Len(t, owner.Rows, 1)
	assert.Equal(t, "exp-1", owner.Rows[0].Values[0].GetString())
}
