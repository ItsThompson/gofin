package dbmigrate_test

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/ItsThompson/gofin/services/dbmigrate"
)

// TestAdminFinanceCleanup proves that the shipping cleanup.sql is correctly
// scoped and idempotent before it is ever run on prod. It is TEMPORARY: the
// completion PR for change-management item 001 removes this file once the
// operation has run (the SQL and markdown remain as the permanent record).
//
// The test is gated on TEST_DATABASE_URL and skips when unset, matching the
// other gated dbmigrate tests. Run it locally against a disposable Postgres:
//
//	TEST_DATABASE_URL='postgres://gofin:gofin@localhost:5432/gofin?sslmode=disable' \
//	  go test ./dbmigrate/ -run TestAdminFinanceCleanup

// cleanupSQLPath points at the exact file that ships in the 001 item, relative
// to services/dbmigrate/, so the test exercises the shipping asset itself.
const cleanupSQLPath = "../../change-management/001_admin-finance-cleanup/assets/cleanup.sql"

// targetTables are the five tables cleanup.sql deletes admin-owned rows from.
// Every one has a user_id column, so counts can be scoped by owner.
var targetTables = []string{
	"finance.pro_rata_schedules",
	"finance.tags",
	"finance.budget_periods",
	"finance.default_settings",
	"datarights.export_jobs",
}

// serviceMigration describes one service's migration run against the shared test
// database: its canonical migrations dir (relative to services/dbmigrate/), the
// schema it owns, and a distinct golang-migrate bookkeeping table so the three
// services' schema_migrations do not collide, mirroring prod's per-service
// search_path model.
type serviceMigration struct {
	dir             string
	searchPath      string
	migrationsTable string
}

var serviceMigrations = []serviceMigration{
	{dir: "../auth/db/migrations", searchPath: "auth", migrationsTable: "schema_migrations_auth"},
	{dir: "../finance/db/migrations", searchPath: "finance", migrationsTable: "schema_migrations_finance"},
	{dir: "../datarights/db/migrations", searchPath: "datarights", migrationsTable: "schema_migrations_datarights"},
}

// wantUserRows is the seeded finance-row count per table for one regular user;
// none of these may be touched by cleanup.sql.
var wantUserRows = map[string]int{
	"finance.pro_rata_schedules": 1,
	"finance.tags":               2,
	"finance.budget_periods":     1,
	"finance.default_settings":   1,
	"datarights.export_jobs":     1,
}

func TestAdminFinanceCleanup(t *testing.T) {
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL not set: skipping integration test")
	}

	runServiceMigrations(t, baseURL)
	db := openDB(t, baseURL)

	adminID := seedUser(t, db, "cleanup_admin", "admin")
	userID := seedUser(t, db, "cleanup_user", "user")
	seedFinanceRows(t, db, adminID)
	seedFinanceRows(t, db, userID)
	auditID := seedDeletionJob(t, db, userID, adminID)

	// Pre-cleanup sanity: the admin owns rows in every target table.
	for _, table := range targetTables {
		if countByUser(t, db, table, adminID) == 0 {
			t.Fatalf("seed check failed: admin owns no rows in %s", table)
		}
	}

	runCleanupSQL(t, db)

	// Scoping: every admin-owned row is gone from the five target tables.
	for _, table := range targetTables {
		if got := countByUser(t, db, table, adminID); got != 0 {
			t.Errorf("admin rows not deleted from %s: got %d, want 0", table, got)
		}
	}

	// Blast radius: regular-user finance rows are untouched.
	for table, want := range wantUserRows {
		if got := countByUser(t, db, table, userID); got != want {
			t.Errorf("regular-user rows in %s changed: got %d, want %d", table, got, want)
		}
	}

	// The admin identity itself must survive (operator accounts must remain).
	if got := queryCount(t, db,
		"SELECT count(*) FROM auth.users WHERE id = $1 AND role = 'admin'", adminID); got != 1 {
		t.Errorf("auth.users admin row was modified: got %d, want 1", got)
	}

	// Audit data in datarights.deletion_jobs must survive.
	if got := queryCount(t, db,
		"SELECT count(*) FROM datarights.deletion_jobs WHERE id = $1", auditID); got != 1 {
		t.Errorf("datarights.deletion_jobs audit row was deleted: got %d, want 1", got)
	}

	// Idempotency: a second run deletes zero rows and leaves the DB unchanged.
	before := snapshot(t, db, adminID, userID, auditID)
	runCleanupSQL(t, db)
	after := snapshot(t, db, adminID, userID, auditID)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("cleanup.sql is not idempotent: state changed on second run\nbefore: %v\nafter: %v", before, after)
	}
}

// runServiceMigrations applies the auth, finance, and datarights migrations to
// the test database, each on its own search_path and bookkeeping table.
func runServiceMigrations(t *testing.T, baseURL string) {
	t.Helper()
	for _, svc := range serviceMigrations {
		dbURL := withParams(t, baseURL, map[string]string{
			"search_path":        svc.searchPath,
			"x-migrations-table": svc.migrationsTable,
		})
		if err := dbmigrate.Run(dbURL, svc.dir); err != nil {
			t.Fatalf("running %s migrations: %v", svc.searchPath, err)
		}
	}
}

// openDB connects for seeding, running cleanup.sql, and asserting. Every query
// uses fully-qualified table names, so the connection carries no search_path.
func openDB(t *testing.T, baseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", stripParams(t, baseURL, "search_path", "x-migrations-table"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("pinging test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedUser inserts an auth.users row with the given role and returns its id.
// The row and everything it owns is removed on cleanup so the test re-runs.
func seedUser(t *testing.T, db *sql.DB, prefix, role string) string {
	t.Helper()
	suffix := time.Now().UnixNano()
	var id string
	err := db.QueryRow(
		`INSERT INTO auth.users (username, email, password_hash, role)
		 VALUES ($1, $2, 'x', $3) RETURNING id::text`,
		fmt.Sprintf("%s_%d", prefix, suffix),
		fmt.Sprintf("%s_%d@cleanup.test", prefix, suffix),
		role,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding %s user: %v", role, err)
	}
	t.Cleanup(func() { deleteUserData(db, id) })
	return id
}

// seedFinanceRows inserts one owner's set of finance rows plus an export job:
// a budget period, default settings, two tags, one pro-rata schedule, one
// export job. finance and datarights carry no cross-schema FK to auth.users
// (project convention), so synthetic tag/group ids are fine.
func seedFinanceRows(t *testing.T, db *sql.DB, userID string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO finance.budget_periods
		(user_id, year, month, budget_amount, essentials_percent, desires_percent, savings_percent)
		VALUES ($1, 2026, 1, 100000, 50, 30, 20)`, userID)
	mustExec(t, db, `INSERT INTO finance.default_settings (user_id) VALUES ($1)`, userID)
	mustExec(t, db, `INSERT INTO finance.tags (user_id, name) VALUES ($1, 'Groceries'), ($1, 'Rent')`, userID)
	mustExec(t, db, `INSERT INTO finance.pro_rata_schedules
		(user_id, pro_rata_group, name, amount, currency, expense_type, tag_id,
		 target_year, target_month, installment_index, installment_total)
		VALUES ($1, gen_random_uuid(), 'Annual insurance', 120000, 'USD', 'essentials',
		 gen_random_uuid(), 2026, 1, 1, 12)`, userID)
	mustExec(t, db, `INSERT INTO datarights.export_jobs (user_id) VALUES ($1)`, userID)
}

// seedDeletionJob inserts an audit row that records the admin acting on a user.
// cleanup.sql must never touch datarights.deletion_jobs. Returns its id.
func seedDeletionJob(t *testing.T, db *sql.DB, userID, adminID string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO datarights.deletion_jobs (user_id, admin_user_id)
		 VALUES ($1, $2) RETURNING id::text`, userID, adminID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding deletion job: %v", err)
	}
	return id
}

// runCleanupSQL reads and executes the exact shipping cleanup.sql file. It has
// no parameters, so lib/pq runs the whole BEGIN; ... COMMIT; as one statement.
func runCleanupSQL(t *testing.T, db *sql.DB) {
	t.Helper()
	content, err := os.ReadFile(cleanupSQLPath)
	if err != nil {
		t.Fatalf("reading cleanup.sql at %s: %v", cleanupSQLPath, err)
	}
	if _, err := db.Exec(string(content)); err != nil {
		t.Fatalf("executing cleanup.sql: %v", err)
	}
}

// snapshot captures the owner-scoped counts the idempotency check compares.
func snapshot(t *testing.T, db *sql.DB, adminID, userID, auditID string) map[string]int {
	t.Helper()
	s := make(map[string]int)
	for _, table := range targetTables {
		s[table+"|admin"] = countByUser(t, db, table, adminID)
		s[table+"|user"] = countByUser(t, db, table, userID)
	}
	s["auth.users|admin"] = queryCount(t, db,
		"SELECT count(*) FROM auth.users WHERE id = $1 AND role = 'admin'", adminID)
	s["datarights.deletion_jobs|audit"] = queryCount(t, db,
		"SELECT count(*) FROM datarights.deletion_jobs WHERE id = $1", auditID)
	return s
}

// deleteUserData removes everything a seeded user owns plus the user row, so a
// crashed prior run never leaves rows that break the next one.
func deleteUserData(db *sql.DB, id string) {
	for _, stmt := range []string{
		"DELETE FROM finance.pro_rata_schedules WHERE user_id = $1",
		"DELETE FROM finance.tags WHERE user_id = $1",
		"DELETE FROM finance.budget_periods WHERE user_id = $1",
		"DELETE FROM finance.default_settings WHERE user_id = $1",
		"DELETE FROM datarights.export_jobs WHERE user_id = $1",
		"DELETE FROM datarights.deletion_jobs WHERE user_id = $1 OR admin_user_id = $1",
		"DELETE FROM auth.users WHERE id = $1",
	} {
		_, _ = db.Exec(stmt, id)
	}
}

// countByUser counts rows a user owns in the given fully-qualified table. The
// table name comes from targetTables (this package), never external input.
func countByUser(t *testing.T, db *sql.DB, table, userID string) int {
	t.Helper()
	return queryCount(t, db, "SELECT count(*) FROM "+table+" WHERE user_id = $1", userID)
}

func queryCount(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count query failed: %v\nquery: %s", err, query)
	}
	return n
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec failed: %v\nquery: %s", err, query)
	}
}

// withParams returns baseURL with the given query params set (added or replaced).
func withParams(t *testing.T, baseURL string, params map[string]string) string {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parsing TEST_DATABASE_URL: %v", err)
	}
	q := parsed.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// stripParams returns baseURL with the given query params removed.
func stripParams(t *testing.T, baseURL string, keys ...string) string {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parsing TEST_DATABASE_URL: %v", err)
	}
	q := parsed.Query()
	for _, k := range keys {
		q.Del(k)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}
