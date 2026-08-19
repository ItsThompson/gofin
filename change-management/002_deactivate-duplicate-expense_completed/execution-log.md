# Execution Log: 002_deactivate-duplicate-expense

- Technician: thompson
- Date/Time started: 2026-08-19 00:15 UTC
- Environment: prod

## Preflight
### Activity 1: Merge the item-creation PR to main
- [x] Confirm the item-creation PR is approved and up to date with `main`.
- [x] Merge the PR to `main`.
- [x] Note the merge commit SHA for the repo-SHA check in Validation 1.
**Validation 1: CI is green on main**
- [x] The `main` CI run for the merge commit is green (build, tests, lint).
- [x] The `validate-change-management` job passed.
> Comments: PR #61 merged to main at 2026-08-19T00:10:40Z. Merge commit: 668aa44f5ecb384fb64c64bfa19d4db7ef261b6e. CI green per user report.

```text
$ gh pr view 61 --repo ItsThompson/gofin --json state,mergedAt,mergeCommit,headRefName,baseRefName,title,url
{"baseRefName":"main","headRefName":"chore/002-deactivate-duplicate-expense","mergeCommit":{"oid":"668aa44f5ecb384fb64c64bfa19d4db7ef261b6e"},"mergedAt":"2026-08-19T00:10:40Z","state":"MERGED","title":"chore(cm): add 002 deactivate duplicate expense","url":"https://github.com/ItsThompson/gofin/pull/61"}
```

### Activity 2: Pull the merged repo on prod and confirm services healthy
- [x] At `/opt/gofin`, run `git fetch origin && git reset --hard origin/main`.
- [x] Record `git rev-parse HEAD` and compare with the merge commit SHA.
- [x] `docker compose ps` shows all services healthy.
**Validation 2: Repo SHA matches and services are healthy**
- [x] `git -C /opt/gofin rev-parse HEAD` matches the merge commit from
- [x] `docker compose ps` reports every service healthy.
- [x] The app responds over its public URL (login page loads).
> Comments: Prod pulled to 668aa44f5ecb384fb64c64bfa19d4db7ef261b6e (matches merge commit). All services healthy. App serves locally (mfe-local HTTP 200); public URL returns 403 to curl (Cloudflare bot protection, not a service fault). User to confirm login page in a real browser.

```text
root@gofin:/opt/gofin# git fetch origin && git reset --hard origin/main && git rev-parse HEAD
HEAD is now at 668aa44 Merge pull request #61 from ItsThompson/chore/002-deactivate-duplicate-expense
668aa44f5ecb384fb64c64bfa19d4db7ef261b6e

root@gofin:/opt/gofin# docker compose ps
# all services Up; api-gateway, auth-service, datarights-service, expense-service,
# finance-service, immudb, postgresql all report (healthy)

root@gofin:/opt/gofin# curl -sS -o /dev/null -w "mfe-local HTTP %{http_code}\n" http://localhost:3000/login
mfe-local HTTP 200
```

### Activity 3: Run the read-only duplicate identification on prod
- [x] Get the owner account id:
- [x] Load immudb credentials: `set -a; source /opt/gofin/.env; set +a`.
- [x] Run the pair query from `assets/find-duplicates.sql` (substitute
- [x] Record both row ids, their `created_at` order, and the immudb credentials
**Validation 3: Exactly two active rows; duplicate identified**
- [x] Exactly two rows returned, both `status = 'active'`.
- [x] Both rows have the same `user_id`, which matches the owner id from
- [x] The rows match on name, amount (2199), and expense_date (2026-08-18);
- [x] The row with the later `created_at` is recorded as the deactivation target
> Comments: Owner is thompson (user). immudb credentials: username immudb, password set (masked). Pair query returned exactly two active rows; pro_rata_group and corrects_id empty (null-equivalent).

```text
root@gofin:/opt/gofin# docker compose exec -T postgresql psql -U gofin -d gofin -c "SELECT id, username, email, role FROM auth.users ORDER BY created_at;"
                  id                  | username |         email          | role
--------------------------------------+----------+------------------------+-------
 40609b4b-2aec-4f20-8d28-9a80d1c53451 | admin    | admin@gofin.local      | admin
 9beac96c-00c6-4f07-84db-f03a2f0ef813 | thompson | itsthompson1@gmail.com | user
(2 rows)

# find-duplicates.sql (USER_ID = 9beac96c-00c6-4f07-84db-f03a2f0ef813)
| id                                     | user_id                                | name                  | amount | expense_date | status  | created_at            |
| a8b0ffa9-c9f6-4f2c-a5d0-10781cdb7725   | 9beac96c-00c6-4f07-84db-f03a2f0ef813   | British Airlines Wifi | 2199   | 2026-08-18   | active  | 2026-08-18T19:55:52Z  |
| a5841a1e-51d4-429a-a79f-fde2c466fbb5   | 9beac96c-00c6-4f07-84db-f03a2f0ef813   | British Airlines Wifi | 2199   | 2026-08-18   | active  | 2026-08-18T19:55:54Z  |
```

## Steps
### Activity 1: SSH to the VPS and enter the repo
- [x] SSH to the production VPS.
- [x] `cd /opt/gofin`.
- [x] Load immudb credentials: `set -a; source /opt/gofin/.env; set +a`.
**Validation 1: Correct host and repo SHA**
- [x] `hostname` (or the cloud console) confirms the intended production host.
- [x] `git rev-parse HEAD` matches the merge commit SHA from preflight.
> Comments: SSH prompt shows root@gofin (77.42.125.184). Repo SHA 668aa44f5ecb384fb64c64bfa19d4db7ef261b6e matches merge commit.

```text
root@gofin:/opt/gofin# git rev-parse HEAD
668aa44f5ecb384fb64c64bfa19d4db7ef261b6e
```

### Activity 2: Record pre-state and time-travel anchor
- [x] Query both rows by id (substitute `<KEPT_ID>` and `<DUP_ID>`):
- [x] Record the current transaction id:
- [x] Note the `<KEPT_ID>`, `<DUP_ID>`, and pre-change TX id in the execution log.
**Validation 2: Pre-state matches preflight**
- [x] The query returns exactly two rows: `<KEPT_ID>` and `<DUP_ID>`.
- [x] Both rows are `status = 'active'` with identical name and amount.
- [x] The TX id is recorded for the time-travel rollback path.
> Comments: KEPT_ID = a8b0ffa9-c9f6-4f2c-a5d0-10781cdb7725 (19:55:52Z), DUP_ID = a5841a1e-51d4-429a-a79f-fde2c466fbb5 (19:55:54Z). Pre-change TX id 1172.

```text
# pair query by id
| id                                     | user_id                                | name                  | amount | expense_date | status  | created_at            |
| a8b0ffa9-c9f6-4f2c-a5d0-10781cdb7725   | 9beac96c-00c6-4f07-84db-f03a2f0ef813   | British Airlines Wifi | 2199   | 2026-08-18   | active  | 2026-08-18T19:55:52Z  |
| a5841a1e-51d4-429a-a79f-fde2c466fbb5   | 9beac96c-00c6-4f07-84db-f03a2f0ef813   | British Airlines Wifi | 2199   | 2026-08-18   | active  | 2026-08-18T19:55:54Z  |

# immuclient current
database:         defaultdb
txID:             1172
hash:             40c71041d79948615d2f1e5d8a27a4139d37eebde612c8727fdcb7329317ee9a
```

### Activity 3: Deactivate the duplicate
- [x] Run the UPDATE:
- [x] Capture the output: expect `Updated rows: 1`.
**Validation 3: Exactly one active row remains**
- [x] The UPDATE output was `Updated rows: 1` (not 0 or more than 1).
- [x] Re-run the pair query from Activity 2: `<KEPT_ID>` is `active` and
- [x] Run `assets/verify.sql` (substitute `<USER_ID>`): exactly one active
> Comments: UPDATE returned `Updated rows: 1`. Pair query shows KEPT_ID active, DUP_ID corrected. verify.sql shows exactly one active row (KEPT_ID) and one corrected row (DUP_ID).

```text
# deactivate-duplicate.sql (DUP_ID = a5841a1e-51d4-429a-a79f-fde2c466fbb5, USER_ID = 9beac96c-00c6-4f07-84db-f03a2f0ef813)
Updated rows: 1

# pair query by id (post-update)
| id                                     | name                  | amount | expense_date | status    | created_at            |
| a8b0ffa9-c9f6-4f2c-a5d0-10781cdb7725   | British Airlines Wifi | 2199   | 2026-08-18   | active     | 2026-08-18T19:55:52Z  |
| a5841a1e-51d4-429a-a79f-fde2c466fbb5   | British Airlines Wifi | 2199   | 2026-08-18   | corrected  | 2026-08-18T19:55:54Z  |

# verify.sql (USER_ID = 9beac96c-00c6-4f07-84db-f03a2f0ef813)
| id                                     | name                  | amount | expense_date | status    | created_at            |
| a8b0ffa9-c9f6-4f2c-a5d0-10781cdb7725   | British Airlines Wifi | 2199   | 2026-08-18   | active     | 2026-08-18T19:55:52Z  |
| a5841a1e-51d4-429a-a79f-fde2c466fbb5   | British Airlines Wifi | 2199   | 2026-08-18   | corrected  | 2026-08-18T19:55:54Z  |
```

### Activity 4: Post-check in the app
- [x] Open the app and sign in as the owner.
- [x] In the expense log for August 2026, confirm a single "British Airlines
- [x] On the dashboard, confirm August spend dropped by £21.99 and the totals
**Validation 4: User-visible state is correct**
- [x] Exactly one "British Airlines Wifi" entry appears in the August expense log.
- [x] The dashboard totals for August exclude the duplicate.
> Comments: User confirmed the August expense log shows a single entry. Dashboard reads active rows live, so it reflects the same single active row.

```text
2026-08-18    British Airlines Wifi    £21.99    desires    Transport    Active
```

### Activity 5: Finalize via the completion PR
- [x] Generate the log:
- [x] Fill in the execution log (technician, date/time, environment, checkboxes,
- [x] `git mv change-management/002_deactivate-duplicate-expense change-management/002_deactivate-duplicate-expense_completed`.
- [x] Open the completion PR with the rename and the filled `execution-log.md`.
**Validation 5: CI green including validate-change-management**
- [x] CI on the completion PR is green.
- [x] The `validate-change-management` job passes for
> Comments: Completion PR #62 opened. All checks passed, including validate-change-management.

```text
$ gh pr checks 62 --repo ItsThompson/gofin
e2e                        pass
lint-backend               pass
lint-frontend              pass
test-backend               pass
test-frontend              pass
validate-change-management pass
```

## Outcome
- [x] All activities and validations completed
> Notes: Deactivation completed as written. UPDATE returned `Updated rows: 1`; post-check confirmed exactly one active row (KEPT_ID) and one corrected row (DUP_ID). User confirmed the August expense log shows a single British Airlines Wifi entry.
