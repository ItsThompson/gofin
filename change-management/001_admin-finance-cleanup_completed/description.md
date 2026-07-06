# Description: 001_admin-finance-cleanup

## Change Event

#### What is the purpose of this activity or change?

Remove all finance data owned by `admin` (operator) accounts as the data-cleanup
step of the operator-only refactor. After the backend refactor, admins can no
longer create finance data; this one-off deletion removes the pre-existing
admin-owned finance rows so the operator identity holds no personal finance
state.

#### What will be required to execute this change?

- The operator-only backend code PR merged to `main` and deployed to prod, so no
  new admin finance rows can be created between deploy and cleanup.
- SSH access to the production VPS, with the repo checked out at `/opt/gofin`.
- Postgres access on the prod host via `docker compose exec postgresql psql`.
- A fresh database backup/snapshot taken immediately before the deletion.
- The committed assets `assets/dry-run.sql` (read-only pre-check) and
  `assets/cleanup.sql` (transactional, admin-scoped delete).

#### What is the expected end state of the system after this change?

Zero admin-owned rows in the four `finance` target tables
(`finance.pro_rata_schedules`, `finance.tags`, `finance.budget_periods`,
`finance.default_settings`) and in `datarights.export_jobs`. Admin operator
accounts in `auth.users` remain intact, and audit data in
`datarights.deletion_jobs` (which references `admin_user_id`) is preserved.
Re-running `assets/dry-run.sql` reports all zeros.

#### What assumptions, if any, are being made about the state of the system at the time of this change?

- Admin is identified by `auth.users.role = 'admin'` in the same `gofin`
  database.
- Production currently holds approximately one admin-owned budget row and no
  admin expense data; the preflight dry-run confirms exact counts before the
  deletion runs.
- All services share one Postgres database `gofin` (differing only by
  `search_path`), so fully-qualified cross-schema deletes run on a single
  connection regardless of `search_path`.
- There are no cross-schema foreign keys by project convention; intra-schema
  delete order (schedules/tags before periods/settings) is safe.

#### Rollout Date/Time(s) and Duration

On demand, once the operator-only backend code PR is deployed to prod. Expected
duration under 30 minutes, including the pre-run backup and all validations. Run
during a low-traffic window.

## Impact / Risk Assessment

#### Why is it necessary? What is the impact of not making this change?

Without it, `admin` (operator) accounts retain personal finance rows that
contradict the operator-only model: the operator identity is meant to own no
personal finance state. Leaving the data in place produces an inconsistent role
model, stale references, and confusing operator accounts.

#### Why does this activity or change need to be done under Change Management? Can it be safely automated?

It is a destructive, one-off production data deletion that cannot be trivially
reversed and has no meaningful backward migration. It should not persist in the
codebase as a startup migration or a `just` recipe. It needs a human executing
checklists with validations, a pre-run backup, and a documented rollback, so it
is a change-managed manual operation rather than an automated migration.

#### Are there any related, prerequisite changes upon which this CM hinges?

It hinges on the operator-only backend refactor PR being merged and deployed to
prod first, so admins can no longer create finance data in the window between the
deploy and the cleanup. It also depends on the committed `assets/dry-run.sql` and
`assets/cleanup.sql`, and on the safety test that proves their scope and
idempotency before merge.

#### Will this CM be in any way intrusive, and if so, how will you know? What teams, services or functionality will be impacted?

The blast radius is limited to admin-owned finance rows. Only operator accounts'
finance data is deleted; regular users' finance data is untouched because every
delete is scoped to admin user ids. Impacted schemas are `finance` and
`datarights`. Impact is detected by comparing the deleted-row counts against the
dry-run and by re-running the dry-run afterward. The deletion is a single atomic
transaction, so a partial failure leaves no partial state.

#### How has this change been tested to verify it's safe for production?

A gated Go integration test (`services/dbmigrate/admin_finance_cleanup_test.go`)
runs the exact shipping `assets/cleanup.sql` against a disposable Postgres. It
seeds both an admin and a regular user with matching finance rows and asserts:
all admin finance rows deleted, all regular-user rows intact, the `auth.users`
admin row intact, `datarights.deletion_jobs` audit rows intact, and a second run
deletes zero rows (idempotent). The preflight dry-run re-confirms scope on prod
immediately before execution.

## Worst Case Scenario

#### What could happen if everything goes wrong with this change?

An incorrectly scoped or mistyped statement deletes finance data belonging to
regular users, or deletes more than intended, causing irreversible loss of user
financial records.

#### How does this CM attempt to mitigate this risk?

- Every delete is filtered to admin user ids via
  `user_id IN (SELECT id FROM auth.users WHERE role = 'admin')`; no regular-user
  row can match.
- All deletes run inside one `BEGIN; ... COMMIT;` transaction, so any error rolls
  the whole operation back with no partial deletion.
- A full database backup/snapshot is taken immediately before execution, enabling
  a restore.
- The scoping and idempotency are proven by the gated integration test, and the
  prod dry-run confirms counts before the deletion runs.

## Rollback Procedure

#### What conditions would indicate a need to rollback?

The deleted-row counts do not match the dry-run; the post-check dry-run shows
remaining admin-owned rows or missing regular-user/audit data; an error occurs
mid-transaction; or a service becomes unhealthy after execution.

#### In the event of problems, what will you do to return your system to a known good state?

Because the deletion is a single atomic transaction, an error before `COMMIT`
leaves the data untouched, so no data action is needed beyond investigating the
cause. If a committed deletion is later found to be wrong, restore the `gofin`
database from the pre-run backup/snapshot taken in `steps.md`. For the code
deploy, roll back to the previous SHA image per `scripts/deploy.sh`. Revert the
item-creation or completion PRs as needed.

#### If this is a software or infrastructure change, has the rollback procedure been verified in a development environment?

The scoping and idempotency were verified by the gated integration test against a
disposable Postgres, including the re-run (no-op) assertion that exercises the
atomic-transaction behavior. The database restore-from-backup path uses the
standard Postgres `pg_dump`/`pg_restore` workflow validated as part of the backup
step in `steps.md`.
