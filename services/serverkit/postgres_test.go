package serverkit_test

import (
	"context"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/serverkit"
)

// TestConnectPostgres_MigrationFailure_ReturnsErrorNoPool exercises the failure
// boundary without a live database: with nothing listening on the target port,
// the migration step fails and ConnectPostgres must surface a wrapped error and
// no pool. The success path (real migrate + ping) is integration-level.
func TestConnectPostgres_MigrationFailure_ReturnsErrorNoPool(t *testing.T) {
	migrations := fstest.MapFS{
		"1_init.up.sql":   &fstest.MapFile{Data: []byte("SELECT 1;")},
		"1_init.down.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}

	// A reserved-then-closed address guarantees a refused connection.
	dbURL := fmt.Sprintf("postgres://user:pass@%s/db?sslmode=disable", freeAddr(t))

	pool, err := serverkit.ConnectPostgres(context.Background(), dbURL, migrations)

	require.Error(t, err)
	assert.Nil(t, pool)
	assert.ErrorContains(t, err, "running migrations")
}
