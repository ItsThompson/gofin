# Steps: 002_deactivate-duplicate-expense

> The execution itself: connect to prod, record the pre-state and time-travel
> anchor, flip the duplicate to `corrected`, post-check, then finalize via the
> completion PR. Each `# Activity N` is paired with a `# Validation N`.
> Preflight (merge, pull, dry-run) is done in `preflight.md` before starting
> here. `<DUP_ID>`, `<KEPT_ID>`, and `<USER_ID>` are the values recorded in
> preflight Activity 3.

# Activity 1: SSH to the VPS and enter the repo

**Description**: Connect to the production VPS and change into the deployed
repo so subsequent commands run against the prod stack.

## Checklist:
1. SSH to the production VPS.
2. `cd /opt/gofin`.
3. Load immudb credentials: `set -a; source /opt/gofin/.env; set +a`.

## Rollback Plan:
1. None: connecting and changing directory make no changes. Disconnect if the
   host or repo state is wrong.

# Validation 1: Correct host and repo SHA

**Description**: Confirm you are on the intended prod host and the repo is at
the merge commit from preflight Activity 1.

## Checklist:
1. `hostname` (or the cloud console) confirms the intended production host.
2. `git rev-parse HEAD` matches the merge commit SHA from preflight.

## Rollback Plan:
1. If the host or SHA is wrong, stop and do not proceed; re-verify the pull
   before continuing.

# Activity 2: Record pre-state and time-travel anchor

**Description**: Re-query the two rows by id to confirm the pre-state, and
record the current immudb transaction id as the time-travel anchor for
rollback inspection. Both commands are read-only.

## Checklist:
1. Query both rows by id (substitute `<KEPT_ID>` and `<DUP_ID>`):
   `docker run --rm --net host codenotary/immuclient:1.11 query --immudb-address 127.0.0.1 --immudb-port 3322 --username "${IMMUDB_USERNAME:-immudb}" --password "${IMMUDB_PASSWORD:-immudb}" --database defaultdb "SELECT id, user_id, name, amount, expense_date, status, created_at FROM expenses WHERE id IN ('<KEPT_ID>', '<DUP_ID>') ORDER BY created_at ASC;"`
2. Record the current transaction id:
   `docker run --rm --net host codenotary/immuclient:1.11 current --immudb-address 127.0.0.1 --immudb-port 3322 --username "${IMMUDB_USERNAME:-immudb}" --password "${IMMUDB_PASSWORD:-immudb}" --database defaultdb`
3. Note the `<KEPT_ID>`, `<DUP_ID>`, and pre-change TX id in the execution log.

## Rollback Plan:
1. None required: both commands are read-only. If the pre-state does not match
   preflight, stop and investigate before the UPDATE.

# Validation 2: Pre-state matches preflight

**Description**: Confirm both rows are still `active` and unchanged since the
preflight dry-run.

## Checklist:
1. The query returns exactly two rows: `<KEPT_ID>` and `<DUP_ID>`.
2. Both rows are `status = 'active'` with identical name and amount.
3. The TX id is recorded for the time-travel rollback path.

## Rollback Plan:
1. If any row changed since preflight (e.g. the user corrected one entry via
   the app), stop: re-assess the pair before executing the UPDATE.

# Activity 3: Deactivate the duplicate

**Description**: Flip the duplicate row (later `created_at`) to
`status='corrected'` using the SQL from `assets/deactivate-duplicate.sql`
(substitute `<DUP_ID>` and `<USER_ID>`). This removes it from the active-only
materialized view while preserving it in the ledger.

## Checklist:
1. Run the UPDATE:
   `docker run --rm --net host codenotary/immuclient:1.11 exec --immudb-address 127.0.0.1 --immudb-port 3322 --username "${IMMUDB_USERNAME:-immudb}" --password "${IMMUDB_PASSWORD:-immudb}" --database defaultdb "UPDATE expenses SET status = 'corrected' WHERE id = '<DUP_ID>' AND user_id = '<USER_ID>';"`
2. Capture the output: expect `Updated rows: 1`.

## Rollback Plan:
1. Run the inverse UPDATE to restore the row:
   `docker run --rm --net host codenotary/immuclient:1.11 exec --immudb-address 127.0.0.1 --immudb-port 3322 --username "${IMMUDB_USERNAME:-immudb}" --password "${IMMUDB_PASSWORD:-immudb}" --database defaultdb "UPDATE expenses SET status = 'active' WHERE id = '<DUP_ID>' AND user_id = '<USER_ID>';"`
2. For inspection without changing data, query the pre-change state via
   time travel: append `BEFORE TX <pre-change-tx>` to the pair SELECT from
   Activity 2.

# Validation 3: Exactly one active row remains

**Description**: Confirm the UPDATE affected exactly one row and the
materialized view now shows a single entry for the pair.

## Checklist:
1. The UPDATE output was `Updated rows: 1` (not 0 or more than 1).
2. Re-run the pair query from Activity 2: `<KEPT_ID>` is `active` and
   `<DUP_ID>` is `corrected`.
3. Run `assets/verify.sql` (substitute `<USER_ID>`): exactly one active
   "British Airlines Wifi" row for 2026-08-18 remains, and it is `<KEPT_ID>`.

## Rollback Plan:
1. If the output was not `Updated rows: 1`, or the post-check is wrong, run
   the inverse UPDATE from Activity 3's rollback plan and investigate.

# Activity 4: Post-check in the app

**Description**: Confirm the user-visible state: the expense log shows one
entry and the August dashboard totals no longer double-count the expense.

## Checklist:
1. Open the app and sign in as the owner.
2. In the expense log for August 2026, confirm a single "British Airlines
   Wifi" entry remains (£21.99).
3. On the dashboard, confirm August spend dropped by £21.99 and the totals
   look correct.

## Rollback Plan:
1. If the app shows incorrect data (two entries or a missing entry), run the
   inverse UPDATE from Activity 3 and investigate before re-running.

# Validation 4: User-visible state is correct

**Description**: Confirm the fix is observable in the product.

## Checklist:
1. Exactly one "British Airlines Wifi" entry appears in the August expense log.
2. The dashboard totals for August exclude the duplicate.

## Rollback Plan:
1. If the app state is wrong, restore the row with the inverse UPDATE and
   complete the item as `failed` with notes.

# Activity 5: Finalize via the completion PR

**Description**: Finalize the change in the repo: generate and fill in
`execution-log.md`, rename the folder with the outcome status suffix, and open
the completion PR.

## Checklist:
1. Generate the log:
   `python3 change-management/.tools/generate_execution_log.py change-management/002_deactivate-duplicate-expense`
2. Fill in the execution log (technician, date/time, environment, checkboxes,
   command output, comments).
3. `git mv change-management/002_deactivate-duplicate-expense change-management/002_deactivate-duplicate-expense_completed`.
4. Open the completion PR with the rename and the filled `execution-log.md`.

## Rollback Plan:
1. Revert the completion PR if any finalization step is wrong; the executed
   data change is unaffected by reverting repo housekeeping.

# Validation 5: CI green including validate-change-management

**Description**: Confirm the completion PR passes CI, including the
change-management validator, which requires a non-empty `execution-log.md` for
a status-suffixed folder.

## Checklist:
1. CI on the completion PR is green.
2. The `validate-change-management` job passes for
   `002_deactivate-duplicate-expense_completed`.

## Rollback Plan:
1. If `validate-change-management` fails, fix the `execution-log.md` or
   folder name on the PR branch and re-run; do not merge until green.
