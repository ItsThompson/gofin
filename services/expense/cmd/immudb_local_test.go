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

// TestInMemoryClient_SelectByIDIsUserScoped locks in the C2 dev-client fix: the
// empty-userID OR-bypass on select-by-id is removed, so the stub can no longer
// return another user's row (or any row) under an empty user scope.
func TestInMemoryClient_SelectByIDIsUserScoped(t *testing.T) {
	ctx := context.Background()
	client := newTestInMemoryClient()

	_, err := client.SQLExec(ctx,
		"INSERT INTO expenses (id, user_id, status) VALUES (@id, @user_id, @status);",
		map[string]interface{}{"id": "exp-1", "user_id": "user-a", "status": "active"})
	require.NoError(t, err)

	const selectByID = "SELECT id, user_id, status FROM expenses WHERE id = @id AND user_id = @user_id;"

	// Empty user scope: the old bypass would have returned user-a's row.
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
