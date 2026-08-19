# Preflight: 002_deactivate-duplicate-expense

> Everything done before the ledger write: merge the item-creation PR, confirm
> prod has the item and is healthy, then run the read-only dry-run that
> identifies the duplicate pair. Each `# Activity N` is paired with a
> `# Validation N`. See `steps.md` for the execution itself.

# Activity 1: Merge the item-creation PR to main

**Description**: Merge the PR that adds this CM item (markdown plus `assets/`
SQL templates; no code) so the repo at `/opt/gofin` can pull it.

## Checklist:
1. Confirm the item-creation PR is approved and up to date with `main`.
2. Merge the PR to `main`.
3. Note the merge commit SHA for the repo-SHA check in Validation 1.

## Rollback Plan:
1. Revert the merge commit with a follow-up PR if the item must be withdrawn.

# Validation 1: CI is green on main

**Description**: Confirm the merged change built and tested cleanly on `main`,
including the `validate-change-management` job.

## Checklist:
1. The `main` CI run for the merge commit is green (build, tests, lint).
2. The `validate-change-management` job passed.

## Rollback Plan:
1. If CI is red on `main`, revert the merge commit and do not proceed.

# Activity 2: Pull the merged repo on prod and confirm services healthy

**Description**: Pull `main` at `/opt/gofin` (CD does this on push to main;
a manual `git fetch && git reset --hard origin/main` is the fallback) so the
item's `assets/` exist on the VPS. No container rebuild is needed because no
code changed.

## Checklist:
1. At `/opt/gofin`, run `git fetch origin && git reset --hard origin/main`.
2. Record `git rev-parse HEAD` and compare with the merge commit SHA.
3. `docker compose ps` shows all services healthy.

## Rollback Plan:
1. None: pulling markdown-only changes is non-destructive. If a service is
   unhealthy, investigate before touching data; do not proceed.

# Validation 2: Repo SHA matches and services are healthy

**Description**: Confirm the prod repo is at the merge commit and the stack is
healthy before any data operation.

## Checklist:
1. `git -C /opt/gofin rev-parse HEAD` matches the merge commit from
   Activity 1.
2. `docker compose ps` reports every service healthy.
3. The app responds over its public URL (login page loads).

## Rollback Plan:
1. If the SHA is wrong or a service is unhealthy, stop; resolve the deploy
   before continuing.

# Activity 3: Run the read-only duplicate identification on prod

**Description**: Identify the owner user id in Postgres, then query the immudb
ledger for the reported duplicate pair. This mutates nothing and records the
exact row ids and created_at order for the steps.

## Checklist:
1. Get the owner account id:
   `docker compose exec -T postgresql psql -U gofin -d gofin -c "SELECT id, username, email, role FROM auth.users ORDER BY created_at;"`
2. Load immudb credentials: `set -a; source /opt/gofin/.env; set +a`.
3. Run the pair query from `assets/find-duplicates.sql` (substitute
   `<USER_ID>`), via the one-shot client:
   `docker run --rm --net host codenotary/immuclient:1.11 query --immudb-address 127.0.0.1 --immudb-port 3322 --username "${IMMUDB_USERNAME:-immudb}" --password "${IMMUDB_PASSWORD:-immudb}" --database defaultdb "<SQL from assets/find-duplicates.sql with <USER_ID> substituted>"`
4. Record both row ids, their `created_at` order, and the immudb credentials
   used.

## Rollback Plan:
1. None required: the dry-run is read-only. If it fails to run, investigate
   connectivity and re-run before proceeding.

# Validation 3: Exactly two active rows; duplicate identified

**Description**: Confirm the pair matches the report and the deactivation
target is unambiguous.

## Checklist:
1. Exactly two rows returned, both `status = 'active'`.
2. Both rows have the same `user_id`, which matches the owner id from
   `auth.users`.
3. The rows match on name, amount (2199), and expense_date (2026-08-18);
   `pro_rata_group` is NULL on both.
4. The row with the later `created_at` is recorded as the deactivation target
   (`<DUP_ID>`); the earlier row is the kept row (`<KEPT_ID>`).

## Rollback Plan:
1. If any check fails (more or fewer rows, another user's data, pro-rata
   linkage), stop: do not run the UPDATE. Investigate and re-run the dry-run
   before proceeding.
