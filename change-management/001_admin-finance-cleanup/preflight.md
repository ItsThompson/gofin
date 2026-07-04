# Preflight: 001_admin-finance-cleanup

> Everything done before the cleanup is executed: merge the operator-only code
> PR, deploy it to prod, and run the read-only dry-run to confirm scope. Each
> `# Activity N` is paired with a `# Validation N`. See `steps.md` for the
> execution itself.

# Activity 1: Merge the operator-only backend PR to main

**Description**: Merge the operator-only refactor PR so that, once deployed,
`admin` accounts can no longer create finance data. This closes the window in
which new admin finance rows could appear before the cleanup runs.

## Checklist:
1. Confirm the operator-only backend PR is approved and up to date with `main`.
2. Merge the PR to `main`.
3. Note the merge commit SHA for the deploy and post-run SHA check.

## Rollback Plan:
1. Revert the merge commit with a follow-up PR if the change must be undone.

# Validation 1: CI is green on main

**Description**: Confirm the merged change built and tested cleanly on `main`,
including the `validate-change-management` job.

## Checklist:
1. The `main` CI run for the merge commit is green (build, tests, lint).
2. The `validate-change-management` job passed.

## Rollback Plan:
1. If CI is red on `main`, revert the merge commit and do not proceed to deploy.

# Activity 2: Deploy the merged code to prod

**Description**: Deploy the merged `main` to the production VPS so the
operator-only backend is live before any data is deleted.

## Checklist:
1. Run the deploy (CD on push to `main`, or `scripts/deploy.sh <server-ip>`).
2. Confirm the deployed SHA matches the merge commit from Activity 1
   (`/opt/gofin/.deployed-sha`).

## Rollback Plan:
1. Roll back to the previous SHA image per `scripts/deploy.sh` (it re-tags the
   prior `sha-<PREV_SHA>` images to `latest` and recreates the stack).

# Validation 2: Prod services healthy and app reachable

**Description**: Confirm the deploy left every service healthy and the app
reachable before touching data.

## Checklist:
1. On the VPS, `cd /opt/gofin && docker compose ps` shows all services healthy.
2. The app responds over its public URL (login page loads).

## Rollback Plan:
1. If any service is unhealthy or the app is unreachable, roll back to the
   previous SHA image per `scripts/deploy.sh` and do not proceed to the dry-run.

# Activity 3: Run the read-only dry-run pre-check on prod

**Description**: Run `assets/dry-run.sql` on prod to report admin-owned row
counts in every cleanup target. This mutates nothing and confirms the scope
before execution.

## Checklist:
1. On the VPS at `/opt/gofin`, run:
   `docker compose exec -T postgresql psql -U gofin -d gofin < change-management/001_admin-finance-cleanup/assets/dry-run.sql`
2. Record the per-table counts for comparison against the deletion in `steps.md`.

## Rollback Plan:
1. None required: the dry-run is read-only. If it fails to run, investigate DB
   connectivity and re-run before proceeding.

# Validation 3: Admin-owned counts match expectation

**Description**: Confirm the dry-run counts match the expected scope (roughly one
admin-owned budget row, all other targets zero).

## Checklist:
1. `finance.budget_periods` reports approximately 1 admin-owned row.
2. `finance.pro_rata_schedules`, `finance.tags`, `finance.default_settings`, and
   `datarights.export_jobs` report the expected counts (0 unless known otherwise).

## Rollback Plan:
1. If any count is unexpected, stop: do not run `cleanup.sql`. Investigate the
   anomaly (e.g. an admin created finance data before deploy) and resolve it
   before re-running the dry-run.
