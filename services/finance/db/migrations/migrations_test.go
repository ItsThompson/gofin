package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationReportingCurrencyBackfillPrecedence verifies the up migration
// SQL implements the required fallback source precedence:
//
//	valid finance.default_settings.currency
//	  → valid auth.users.currency
//	  → configured app fallback currency
//
// and that the migration report records rows that used auth fallback or app
// fallback (ticket #11 AC2, AC11).
func TestMigrationReportingCurrencyBackfillPrecedence(t *testing.T) {
	upSQL, err := FS.ReadFile("000006_add_period_reporting_currency.up.sql")
	require.NoError(t, err)
	sql := string(upSQL)

	// Step 1: finance_default backfill must come before auth_user backfill.
	financeDefaultIdx := strings.Index(sql, "'finance_default'")
	authUserIdx := strings.Index(sql, "'auth_user'")
	require.True(t, financeDefaultIdx >= 0, "migration must have a finance_default source")
	require.True(t, authUserIdx >= 0, "migration must have an auth_user source")
	assert.Less(t, financeDefaultIdx, authUserIdx,
		"finance_default backfill must precede auth_user backfill")

	// Step 2: auth_user backfill must come before app fallback.
	fallbackIdx := strings.Index(sql, "'fallback'")
	require.True(t, fallbackIdx >= 0, "migration must have a fallback source")
	assert.Less(t, authUserIdx, fallbackIdx,
		"auth_user backfill must precede app fallback")

	// Step 3: auth_user backfill uses NOT EXISTS to skip rows already backfilled
	// from finance defaults (precedence enforcement).
	authSection := sql[authUserIdx:fallbackIdx]
	assert.Contains(t, authSection, "NOT EXISTS",
		"auth_user backfill must skip rows already filled by finance_default")

	// Step 4: fallback also uses NOT EXISTS to skip rows already backfilled.
	fallbackSection := sql[fallbackIdx:]
	assert.Contains(t, fallbackSection, "NOT EXISTS",
		"app fallback must skip rows already filled by finance_default or auth_user")

	// Step 5: migration report records both auth_user and fallback rows.
	assert.Contains(t, sql, "'auth_fallback'",
		"migration report must record auth fallback rows")
	assert.Contains(t, sql, "'app_fallback'",
		"migration report must record app fallback rows")
	assert.Contains(t, sql, "backfill.source IN ('auth_user', 'fallback')",
		"migration report must filter for auth_user and fallback sources")

	// Step 6: validation checks every row has a supported reporting currency.
	assert.Contains(t, sql, "RAISE EXCEPTION",
		"migration must validate every row has a supported reporting currency")

	// Step 7: NOT NULL constraint is added.
	assert.Contains(t, sql, "SET NOT NULL",
		"migration must add NOT NULL constraint on reporting_currency")
}

// TestMigrationReportingCurrencyDownDocumentsHistoryLoss verifies the down
// migration documents that it does not preserve period-currency history
// (ticket #11 AC4).
func TestMigrationReportingCurrencyDownDocumentsHistoryLoss(t *testing.T) {
	downSQL, err := FS.ReadFile("000006_add_period_reporting_currency.down.sql")
	require.NoError(t, err)
	sql := string(downSQL)

	assert.True(t,
		strings.Contains(strings.ToUpper(sql), "DOES NOT PRESERVE") ||
			strings.Contains(strings.ToUpper(sql), "NOT PRESERVE") ||
			strings.Contains(strings.ToUpper(sql), "DOESN'T PRESERVE") ||
			strings.Contains(strings.ToUpper(sql), "DATA LOSS") ||
			strings.Contains(strings.ToUpper(sql), "HISTORY IS LOST") ||
			strings.Contains(strings.ToUpper(sql), "LOST"),
		"down migration must document that it does not preserve period-currency history")

	assert.Contains(t, sql, "DROP COLUMN reporting_currency",
		"down migration must drop the reporting_currency column")
	assert.Contains(t, sql, "DROP TABLE IF EXISTS finance.period_reporting_currency_migration_report",
		"down migration must drop the migration report table")
}
