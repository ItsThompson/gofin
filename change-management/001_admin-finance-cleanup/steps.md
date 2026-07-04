# Steps: 001_admin-finance-cleanup

> The execution itself: connect to prod, back up the database, run the
> transactional cleanup, post-check, then finalize via the completion PR. Each
> `# Activity N` is paired with a `# Validation N`. Preflight (merge, deploy,
> dry-run) is done in `preflight.md` before starting here.

# Activity 1: SSH to the VPS and enter the repo

**Description**: Connect to the production VPS and change into the deployed repo
so subsequent `docker compose` commands run against the prod stack.

## Checklist:
1. SSH to the production VPS.
2. `cd /opt/gofin`.

## Rollback Plan:
1. None: connecting and changing directory make no changes. Disconnect if the
   host or repo state is wrong.

# Validation 1: Correct host and repo SHA

**Description**: Confirm you are on the intended prod host and the deployed SHA
matches the change deployed in preflight.

## Checklist:
1. `hostname` (or the cloud console) confirms the intended production host.
2. `cat /opt/gofin/.deployed-sha` matches the merge commit SHA from preflight
   Activity 1.

## Rollback Plan:
1. If the host or SHA is wrong, stop and do not proceed; re-verify the deploy
   before continuing.

# Activity 2: Take a database backup/snapshot

**Description**: Take a fresh, restorable backup of the `gofin` database
immediately before the deletion so the state can be restored if needed.

## Checklist:
1. At `/opt/gofin`, dump the database, e.g.:
   `mkdir -p backups && docker compose exec -T postgresql pg_dump -U gofin -Fc gofin > backups/gofin-pre-001-$(date +%Y%m%d%H%M%S).dump`
2. Record the backup artifact path and size.

## Rollback Plan:
1. None: taking a backup is non-destructive. If the dump fails, do not proceed to
   the cleanup until a valid backup exists.

# Validation 2: Backup artifact exists and is restorable

**Description**: Confirm the backup was written and is a valid, restorable
archive before any data is deleted.

## Checklist:
1. The dump file exists and is non-empty.
2. `pg_restore -l <dump>` (or a restore into a scratch database) lists the
   archive contents without error.

## Rollback Plan:
1. If the backup is missing or unreadable, stop and re-take it; do not run
   `cleanup.sql` without a verified backup.

# Activity 3: Run cleanup.sql in a transaction

**Description**: Run `assets/cleanup.sql` against prod Postgres. It wraps the
five admin-scoped deletes in a single `BEGIN; ... COMMIT;` so the operation is
atomic.

## Checklist:
1. At `/opt/gofin`, run:
   `docker compose exec -T postgresql psql -U gofin -d gofin < change-management/001_admin-finance-cleanup/assets/cleanup.sql`
2. Capture the per-statement `DELETE <n>` row counts and the final `COMMIT`.

## Rollback Plan:
1. Any error before `COMMIT` rolls the whole transaction back automatically (no
   partial deletion): investigate and re-run.
2. If a committed deletion is later found to be wrong, restore the `gofin`
   database from the Activity 2 backup with `pg_restore`.

# Validation 3: Deleted-row counts match the dry-run

**Description**: Confirm the deleted counts match the admin-owned counts reported
by the preflight dry-run.

## Checklist:
1. The `DELETE` counts per table equal the preflight dry-run counts (e.g.
   `finance.budget_periods` = approximately 1, others as reported).
2. The transaction ended with `COMMIT` (no rollback/error).

## Rollback Plan:
1. If counts do not match or the transaction errored, do not finalize: restore
   from the Activity 2 backup if needed and investigate before re-running.

# Activity 4: Post-check by re-running the dry-run

**Description**: Re-run `assets/dry-run.sql` to confirm the cleanup removed every
admin-owned finance row and left audit data intact.

## Checklist:
1. At `/opt/gofin`, run:
   `docker compose exec -T postgresql psql -U gofin -d gofin < change-management/001_admin-finance-cleanup/assets/dry-run.sql`
2. Spot-check that `auth.users` admin rows and `datarights.deletion_jobs` audit
   rows are still present.

## Rollback Plan:
1. If admin-owned finance rows remain or expected audit/user data is missing,
   restore from the Activity 2 backup and investigate; complete the item as
   `failed` rather than `completed`.

# Validation 4: Zero admin-owned rows; audit data intact

**Description**: Confirm the end state matches the expected outcome.

## Checklist:
1. The dry-run reports 0 admin-owned rows for all five target tables.
2. `auth.users` admin (operator) rows and `datarights.deletion_jobs` audit rows
   are intact.

## Rollback Plan:
1. If any check fails, restore from the Activity 2 backup and complete the item
   as `failed` with notes; do not rename the folder `completed`.

# Activity 5: Finalize via the completion PR

**Description**: Finalize the change in the repo: remove the temporary safety
test, add the filled-in `execution-log.md`, and rename the folder with the
outcome status suffix.

## Checklist:
1. Generate and fill in `execution-log.md`
   (`python3 change-management/.tools/generate_execution_log.py change-management/001_admin-finance-cleanup`).
2. Remove the temporary safety test
   `services/dbmigrate/admin_finance_cleanup_test.go`.
3. `git mv change-management/001_admin-finance-cleanup change-management/001_admin-finance-cleanup_completed`.
4. Open the completion PR with the rename and the filled `execution-log.md`.

## Rollback Plan:
1. Revert the completion PR if any finalization step is wrong; the executed data
   change is unaffected by reverting repo housekeeping.

# Validation 5: CI green including validate-change-management

**Description**: Confirm the completion PR passes CI, including the
change-management validator, which requires a non-empty `execution-log.md` for a
status-suffixed folder.

## Checklist:
1. CI on the completion PR is green.
2. The `validate-change-management` job passes for
   `001_admin-finance-cleanup_completed`.

## Rollback Plan:
1. If `validate-change-management` fails, fix the `execution-log.md` or folder
   name on the PR branch and re-run; do not merge until green.
