# Execution Log: 001_admin-finance-cleanup

- Technician: thompson
- Date/Time started: 2026-07-06 19:27:51 BST
- Environment: prod

## Preflight
### Activity 1: Merge the operator-only backend PR to main
- [x] Confirm the operator-only backend PR is approved and up to date with `main`.
- [x] Merge the PR to `main`.
- [x] Note the merge commit SHA for the deploy and post-run SHA check.
**Validation 1: CI is green on main**
- [x] The `main` CI run for the merge commit is green (build, tests, lint).
- [x] The `validate-change-management` job passed.
> Comments: PR #35 merged to main at 2026-07-06T08:28:03Z. Merge commit: cc5c2801eaa29214af66f09d4c4ef89c3348c33e. CI green per handoff/user report.

```text
$ gh pr view 35 --repo ItsThompson/gofin --json state,mergedAt,mergeCommit,headRefName,baseRefName,title,url
{"baseRefName":"main","headRefName":"refactor/admin-account","mergeCommit":{"oid":"cc5c2801eaa29214af66f09d4c4ef89c3348c33e"},"mergedAt":"2026-07-06T08:28:03Z","state":"MERGED","title":"refactor(admin): operator-only admin + change-management framework","url":"https://github.com/ItsThompson/gofin/pull/35"}

$ python3 change-management/.validation/validate_change_management.py
OK: 2 change-management item(s) conform.
```

### Activity 2: Deploy the merged code to prod
- [x] Run the deploy (CD on push to `main`, or `scripts/deploy.sh <server-ip>`).
- [x] Confirm the deployed SHA matches the merge commit from Activity 1
**Validation 2: Prod services healthy and app reachable**
- [x] On the VPS, `cd /opt/gofin && docker compose ps` shows all services healthy.
- [x] The app responds over its public URL (login page loads).
> Comments: User confirmed deployed SHA, service health, and public login page.

```text
root@gofin:/opt/gofin# cat .deployed-sha
cc5c2801eaa29214af66f09d4c4ef89c3348c33e

root@gofin:/opt/gofin# docker compose ps
NAME                          IMAGE                                                 COMMAND                  SERVICE               CREATED        STATUS                  PORTS
gofin-alertmanager-1          ghcr.io/itsthompson/gofin/alertmanager:latest         "/entrypoint.sh --co…"   alertmanager          10 hours ago   Up 10 hours             0.0.0.0:9093->9093/tcp, [::]:9093->9093/tcp
gofin-api-gateway-1           ghcr.io/itsthompson/gofin/api-gateway:latest          "/service"               api-gateway           10 hours ago   Up 10 hours (healthy)   0.0.0.0:8080->8080/tcp, [::]:8080->8080/tcp
gofin-auth-service-1          ghcr.io/itsthompson/gofin/auth-service:latest         "/service"               auth-service          10 hours ago   Up 10 hours (healthy)   8081/tcp, 9081/tcp
gofin-cadvisor-1              gcr.io/cadvisor/cadvisor:v0.55.1                      "/usr/bin/cadvisor -…"   cadvisor              8 weeks ago    Up 8 weeks (healthy)    8080/tcp
gofin-cloudflared-app-1       cloudflare/cloudflared:2025.4.0                       "cloudflared --no-au…"   cloudflared-app       7 weeks ago    Up 7 weeks
gofin-cloudflared-grafana-1   cloudflare/cloudflared:2025.4.0                       "cloudflared --no-au…"   cloudflared-grafana   7 weeks ago    Up 7 weeks
gofin-datarights-service-1    ghcr.io/itsthompson/gofin/datarights-service:latest   "/service"               datarights-service    10 hours ago   Up 10 hours (healthy)   8084/tcp, 9084/tcp
gofin-expense-service-1       ghcr.io/itsthompson/gofin/expense-service:latest      "/service"               expense-service       10 hours ago   Up 10 hours (healthy)   8082/tcp, 9082/tcp
gofin-finance-service-1       ghcr.io/itsthompson/gofin/finance-service:latest      "/service"               finance-service       10 hours ago   Up 10 hours (healthy)   8083/tcp, 9083/tcp
gofin-grafana-1               grafana/grafana:11.6.0                                "/run.sh"                grafana               7 weeks ago    Up 7 weeks              0.0.0.0:3001->3000/tcp, [::]:3001->3000/tcp
gofin-grafana-auth-proxy-1    ghcr.io/itsthompson/gofin/grafana-auth-proxy:latest   "/service"               grafana-auth-proxy    10 hours ago   Up 10 hours             0.0.0.0:3002->3002/tcp, [::]:3002->3002/tcp
gofin-immudb-1                codenotary/immudb:1.11.0                              "/usr/sbin/immudb"       immudb                7 weeks ago    Up 4 weeks (healthy)    5432/tcp, 8080/tcp, 0.0.0.0:3322->3322/tcp, [::]:3322->3322/tcp, 9497/tcp
gofin-mfe-1                   ghcr.io/itsthompson/gofin/mfe:latest                  "docker-entrypoint.s…"   mfe                   10 hours ago   Up 10 hours             0.0.0.0:3000->3000/tcp, [::]:3000->3000/tcp
gofin-node-exporter-1         prom/node-exporter:v1.11.1                            "/bin/node_exporter …"   node-exporter         8 weeks ago    Up 8 weeks              9100/tcp
gofin-postgresql-1            postgres:16.8-alpine                                  "docker-entrypoint.s…"   postgresql            4 weeks ago    Up 4 weeks (healthy)    5432/tcp
gofin-prometheus-1            prom/prometheus:v3.11.3                               "/bin/prometheus --c…"   prometheus            7 weeks ago    Up 7 weeks              0.0.0.0:9090->9090/tcp, [::]:9090->9090/tcp
```

User confirmed `usegofin.com/login` loads.

### Activity 3: Run the read-only dry-run pre-check on prod
- [x] On the VPS at `/opt/gofin`, run:
- [x] Record the per-table counts for comparison against the deletion in `steps.md`.
**Validation 3: Admin-owned counts match expectation**
- [x] `finance.budget_periods` reports approximately 1 admin-owned row.
- [x] `finance.pro_rata_schedules`, `finance.tags`, `finance.default_settings`, and
> Comments: Dry-run output and follow-up read-only tag inspection.

```text
root@gofin:/opt/gofin# docker compose exec -T postgresql psql -U gofin -d gofin < change-management/001_admin-finance-cleanup/assets/dry-run.sql
WARNING:  database "gofin" has no actual collation version, but a version was recorded
         table_name         | count
----------------------------+-------
 finance.pro_rata_schedules |     0
 finance.tags               |    10
 finance.budget_periods     |     1
 finance.default_settings   |     1
 datarights.export_jobs     |     0
(5 rows)

root@gofin:/opt/gofin# docker compose exec -T postgresql psql -U gofin -d gofin -c "
   SELECT t.name, t.is_default
   FROM finance.tags t
   JOIN auth.users u ON u.id = t.user_id
   WHERE u.role = 'admin'
   ORDER BY t.name;
   "
WARNING:  database "gofin" has no actual collation version, but a version was recorded
           name           | is_default
--------------------------+------------
 Bills                    | t
 Food                     | t
 Household                | t
 Investment               | t
 Personal Care            | t
 Recreation/Entertainment | t
 Self Investment          | t
 Social                   | t
 Transport                | t
 Travel                   | t
(10 rows)
```

User accepted `finance.tags=10` and `finance.default_settings=1` as expected admin onboarding/default-tag residue. Repo-only subagent review corroborated this: onboarding creates one default settings row and 10 default tags, and the deployed refactor blocks new direct-admin finance rows.

## Steps
### Activity 1: SSH to the VPS and enter the repo
- [x] SSH to the production VPS.
- [x] `cd /opt/gofin`.
**Validation 1: Correct host and repo SHA**
- [x] `hostname` (or the cloud console) confirms the intended production host.
- [x] `cat /opt/gofin/.deployed-sha` matches the merge commit SHA from preflight
> Comments: SSH prompt/output shows the production host and repo path, with deployed SHA matching the merge commit.

```text
root@gofin:/opt/gofin# cat .deployed-sha
cc5c2801eaa29214af66f09d4c4ef89c3348c33e
```

### Activity 2: Take a database backup/snapshot
- [x] At `/opt/gofin`, dump the database, e.g.:
- [x] Record the backup artifact path and size.
**Validation 2: Backup artifact exists and is restorable**
- [x] The dump file exists and is non-empty.
- [x] `pg_restore -l <dump>` (or a restore into a scratch database) lists the
> Comments: Backup created and validated using container `pg_restore` because host `pg_restore` was not installed.

```text
root@gofin:/opt/gofin# mkdir -p backups && docker compose exec -T postgresql pg_dump -U gofin -Fc gofin > backups/gofin-pre-001-$(date +%Y%m%d%H%M%S).dump
WARNING:  database "gofin" has no actual collation version, but a version was recorded

root@gofin:/opt/gofin# ls -lh backups/gofin-pre-001-*.dump | tail -1
-rw-r--r-- 1 root root 24K Jul  6 18:36 backups/gofin-pre-001-20260706183601.dump

root@gofin:/opt/gofin# pg_restore -l backups/gofin-pre-001-20260706183601.dump | head
Command 'pg_restore' not found, but can be installed with:
apt install postgresql-client-common

root@gofin:/opt/gofin# ls -lh backups/gofin-pre-001-20260706183601.dump
-rw-r--r-- 1 root root 24K Jul  6 18:36 backups/gofin-pre-001-20260706183601.dump

root@gofin:/opt/gofin# docker compose exec -T postgresql pg_restore -l < backups/gofin-pre-001-20260706183601.dump >/dev/null && echo "pg_restore list OK"
pg_restore list OK

root@gofin:/opt/gofin# docker compose exec -T postgresql pg_restore -l < backups/gofin-pre-001-20260706183601.dump | head -20
;
; Archive created at 2026-07-06 18:36:01 UTC
;     dbname: gofin
;     TOC Entries: 57
;     Compression: gzip
;     Dump Version: 1.15-0
;     Format: CUSTOM
;     Integer: 4 bytes
;     Offset: 8 bytes
;     Dumped from database version: 16.8
;     Dumped by pg_dump version: 16.8
;
;
; Selected TOC Entries:
;
6; 2615 16385 SCHEMA - auth gofin
8; 2615 16478 SCHEMA - datarights gofin
7; 2615 16386 SCHEMA - finance gofin
219; 1259 16407 TABLE auth refresh_token_blacklist gofin
226; 1259 16507 TABLE auth schema_migrations gofin
```

### Activity 3: Run cleanup.sql in a transaction
- [x] At `/opt/gofin`, run:
- [x] Capture the per-statement `DELETE <n>` row counts and the final `COMMIT`.
**Validation 3: Deleted-row counts match the dry-run**
- [x] The `DELETE` counts per table equal the preflight dry-run counts (e.g.
- [x] The transaction ended with `COMMIT` (no rollback/error).
> Comments: Cleanup completed with `COMMIT`. DELETE counts matched dry-run in cleanup SQL order: `finance.pro_rata_schedules=0`, `finance.tags=10`, `finance.budget_periods=1`, `finance.default_settings=1`, `datarights.export_jobs=0`.

```text
root@gofin:/opt/gofin# docker compose exec -T postgresql psql -U gofin -d gofin < change-management/001_admin-finance-cleanup/assets/cleanup.sql
WARNING:  database "gofin" has no actual collation version, but a version was recorded
BEGIN
DELETE 0
DELETE 10
DELETE 1
DELETE 1
DELETE 0
COMMIT
```

### Activity 4: Post-check by re-running the dry-run
- [x] At `/opt/gofin`, run:
- [x] Spot-check that `auth.users` admin rows and `datarights.deletion_jobs` audit
**Validation 4: Zero admin-owned rows; audit data intact**
- [x] The dry-run reports 0 admin-owned rows for all five target tables.
- [x] `auth.users` admin (operator) rows and `datarights.deletion_jobs` audit rows
> Comments: Post-check dry-run returned zero for all cleanup targets. Spot-check found one admin user remains. `datarights.deletion_jobs` count was zero; no audit rows were present to preserve, and `cleanup.sql` does not target that table.

```text
root@gofin:/opt/gofin# docker compose exec -T postgresql psql -U gofin -d gofin < change-management/001_admin-finance-cleanup/assets/dry-run.sql
WARNING:  database "gofin" has no actual collation version, but a version was recorded
         table_name         | count
----------------------------+-------
 finance.pro_rata_schedules |     0
 finance.tags               |     0
 finance.budget_periods     |     0
 finance.default_settings   |     0
 datarights.export_jobs     |     0
(5 rows)

root@gofin:/opt/gofin# docker compose exec -T postgresql psql -U gofin -d gofin -c "
   SELECT 'auth.admin_users' AS check_name, count(*) FROM auth.users WHERE role = 'admin'
   UNION ALL
   SELECT 'datarights.deletion_jobs', count(*) FROM datarights.deletion_jobs;
   "
WARNING:  database "gofin" has no actual collation version, but a version was recorded
        check_name        | count
--------------------------+-------
 auth.admin_users         |     1
 datarights.deletion_jobs |     0
(2 rows)
```

### Activity 5: Finalize via the completion PR
- [x] Generate and fill in `execution-log.md`
- [x] Remove the temporary safety test
- [x] `git mv change-management/001_admin-finance-cleanup change-management/001_admin-finance-cleanup_completed`.
- [ ] Open the completion PR with the rename and the filled `execution-log.md`.
**Validation 5: CI green including validate-change-management**
- [ ] CI on the completion PR is green.
- [ ] The `validate-change-management` job passes for
> Comments: Repo finalization prepared on branch `chore/complete-001-cleanup`: execution log filled, temporary safety test removed, folder renamed with `_completed` status. Local validation passed.

```text
$ python3 change-management/.validation/validate_change_management.py
OK: 2 change-management item(s) conform.

$ cd services/dbmigrate && go test ./...
ok  	github.com/ItsThompson/gofin/services/dbmigrate	0.269s
```

No `docs/data-model.md` link to `001_admin-finance-cleanup/description.md` was present in the current tree, so no doc-link update was needed.

## Outcome
- [x] Production cleanup activities and validations completed
- Status: completed
> Notes: Cleanup completed as written. Post-check returned zero admin-owned rows for all five target tables. User also verified in production that the admin account and a regular user account are working. Completion PR will rename the folder to `change-management/001_admin-finance-cleanup_completed`, remove the temporary safety test, and rely on CI to re-validate the completed change-management item.
