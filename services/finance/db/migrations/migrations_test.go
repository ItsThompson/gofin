package migrations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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

// TestMigrationReportingCurrencyCodesMatchSharedCatalog asserts the supported-
// currency codes hardcoded in the migration (temp table and CHECK constraint)
// are exactly the codes in shared/currency/catalog.json. The catalog is the
// single source of truth (spec 04); this test flags drift so a future catalog
// change does not silently diverge the migration's validation/CHECK from the
// shared catalog (review S5).
func TestMigrationReportingCurrencyCodesMatchSharedCatalog(t *testing.T) {
	upSQL, err := FS.ReadFile("000006_add_period_reporting_currency.up.sql")
	require.NoError(t, err)
	sql := string(upSQL)

	// All catalog codes are exactly three uppercase letters. Other quoted SQL
	// literals ('finance_default', 'auth_user', 'fallback', etc.) are longer and
	// do not match this pattern, so the regex isolates the currency code list.
	codePattern := regexp.MustCompile(`'([A-Z]{3})'`)
	sqlCodes := map[string]bool{}
	for _, match := range codePattern.FindAllStringSubmatch(sql, -1) {
		sqlCodes[match[1]] = true
	}

	catalogCodes := loadCatalogCodes(t)

	require.Len(t, sqlCodes, len(catalogCodes),
		"SQL currency codes must match the shared catalog size")
	for code := range catalogCodes {
		assert.True(t, sqlCodes[code], "migration SQL missing catalog code %s", code)
	}
	for code := range sqlCodes {
		assert.True(t, catalogCodes[code], "migration SQL contains non-catalog code %s", code)
	}
}

// catalogSourceEntry is the subset of shared/currency/catalog.json the drift
// guard needs.
type catalogSourceEntry struct {
	Code string `json:"code"`
}

// loadCatalogCodes reads the shared currency catalog JSON via the test file's
// absolute path, mirroring the shared/currency contract-test pattern.
func loadCatalogCodes(t *testing.T) map[string]bool {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating migrations test file")
	}

	// migrations_test.go lives at services/finance/db/migrations/; walk up to the
	// repo root then into shared/currency/catalog.json.
	catalogPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "shared", "currency", "catalog.json")
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("reading shared currency catalog: %v", err)
	}

	var entries []catalogSourceEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("parsing shared currency catalog: %v", err)
	}

	codes := make(map[string]bool, len(entries))
	for _, entry := range entries {
		codes[entry.Code] = true
	}
	return codes
}
