package repository

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitSchema_ReconcilesSnapshotColumns asserts InitSchema issues the ALTER
// TABLE ADD COLUMN statements for the money snapshot columns and does not fail
// when the columns already exist (the recording client returns errors that the
// reconcile path swallows).
func TestInitSchema_ReconcilesSnapshotColumns(t *testing.T) {
	client := newRecordingImmudbClient()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	require.NoError(t, repo.InitSchema(context.Background()))

	assert.Equal(t, 9, client.countQueriesContaining("ALTER TABLE EXPENSES ADD COLUMN"),
		"expected one ALTER per snapshot/idempotency column")
}

// addColumnFailingImmudbClient wraps the recording client and fails every
// ALTER TABLE ADD COLUMN SQLExec with "column already exists", modelling a
// table that already has the snapshot columns.
type addColumnFailingImmudbClient struct {
	*recordingImmudbClient
}

func (c *addColumnFailingImmudbClient) SQLExec(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	if strings.Contains(strings.ToUpper(sql), "ALTER TABLE") {
		c.record(sql, params)
		return nil, errors.New("column already exists")
	}
	return c.recordingImmudbClient.SQLExec(ctx, sql, params)
}

// TestInitSchema_SwallowsAddColumnExistsError asserts the reconcile ALTER path
// is idempotent: when the snapshot columns already exist, InitSchema still
// succeeds and issues all ADD COLUMN statements.
func TestInitSchema_SwallowsAddColumnExistsError(t *testing.T) {
	client := &addColumnFailingImmudbClient{recordingImmudbClient: newRecordingImmudbClient()}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	require.NoError(t, repo.InitSchema(context.Background()))

	assert.Equal(t, 9, client.countQueriesContaining("ALTER TABLE EXPENSES ADD COLUMN"),
		"expected all 9 ADD COLUMN statements to be issued even when they fail")
}

// --- InitSchema reconciles idempotency_key column ---

func TestInitSchema_ReconcilesIdempotencyKeyColumn(t *testing.T) {
	client := newRecordingImmudbClient()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	repo := NewImmudbExpenseRepository(client, logger)

	require.NoError(t, repo.InitSchema(context.Background()))

	// The idempotency_key ALTER is issued as part of the reconcile loop.
	assert.Equal(t, 1, client.countQueriesContaining("ALTER TABLE EXPENSES ADD COLUMN IDEMPOTENCY_KEY"),
		"expected an ALTER for the idempotency_key column")
	// The idempotency-key lookup index is created.
	assert.Equal(t, 1, client.countQueriesContaining("IDX_EXPENSES_USER_IDEM"),
		"expected the idempotency-key lookup index to be created")
}
