package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnboardingCompletedAtMigrationBackfillsCompletedUsersWithOneTimestamp(t *testing.T) {
	sqlBytes, err := FS.ReadFile("000003_add_onboarding_completed_at.up.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	assert.Contains(t, sql, "ADD COLUMN onboarding_completed_at TIMESTAMPTZ")
	assert.Contains(t, sql, "WITH completion_time AS")
	assert.Contains(t, sql, "SELECT now() AS completed_at")
	assert.Contains(t, sql, "WHERE users.has_completed_onboarding")
	assert.Contains(t, sql, "onboarding_completed_at IS NULL")
	assert.Contains(t, sql, "CREATE INDEX idx_users_reporting_created_at ON auth.users (created_at)")
	assert.Contains(t, sql, "CREATE INDEX idx_users_reporting_onboarding_completed_at ON auth.users (onboarding_completed_at)")
}

func TestOnboardingCompletedAtMigrationDownDropsNullableColumn(t *testing.T) {
	sqlBytes, err := FS.ReadFile("000003_add_onboarding_completed_at.down.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	assert.Contains(t, strings.ToUpper(sql), "DROP INDEX IF EXISTS AUTH.IDX_USERS_REPORTING_CREATED_AT")
	assert.Contains(t, strings.ToUpper(sql), "DROP INDEX IF EXISTS AUTH.IDX_USERS_REPORTING_ONBOARDING_COMPLETED_AT")
	assert.Contains(t, strings.ToUpper(sql), "DROP COLUMN ONBOARDING_COMPLETED_AT")
}
