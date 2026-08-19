# Description: 002_deactivate-duplicate-expense

## Change Event

#### What is the purpose of this activity or change?

Deactivate one of two identical "British Airlines Wifi" expense rows
(2026-08-18, £21.99) in the prod immudb ledger. A client-side retry after an
internet failure submitted the same expense twice. The duplicate is marked
`status='corrected'` so the active-only materialized view shows a single entry.

#### What will be required to execute this change?

- SSH access to the production VPS, with the repo checked out at `/opt/gofin`.
- Docker on the VPS, to run the one-shot `codenotary/immuclient:1.11`
  container (host network, immudb port 3322 published on the host).
- immudb credentials `IMMUDB_USERNAME` / `IMMUDB_PASSWORD` from
  `/opt/gofin/.env`.
- The committed `assets/` SQL templates (find / deactivate / verify).
- The owner account id from Postgres `auth.users`, used to scope the queries.

#### What is the expected end state of the system after this change?

Exactly one active "British Airlines Wifi" row for 2026-08 remains for the
owner. The duplicate row stays in the ledger with `status='corrected'`;
immudb history is preserved. The expense log and the August dashboard totals,
which read active rows live, show the expense once.

#### What assumptions, if any, are being made about the state of the system at the time of this change?

- The duplicate pair matches on name, amount (2199), expense_date
  (2026-08-18), and user; the rows differ only in id and created_at.
  Preflight confirms exactly two active rows and records both ids.
- The row with the later `created_at` is the retry duplicate and is the one
  deactivated; the earlier row is kept. The rows are identical, so which one
  is kept does not affect totals.
- The entry is a plain (non-pro-rata) expense: preflight checks
  `pro_rata_group` is NULL on both rows.
- August 2026 is the current period, so no stored period summary or health
  score needs recomputation: finance reads active expenses live.

#### Rollout Date/Time(s) and Duration

On demand, after the item-creation PR is merged and the repo pulled at
`/opt/gofin`. Expected duration under 20 minutes. Run during a low-traffic
window.

## Impact / Risk Assessment

#### Why is it necessary? What is the impact of not making this change?

The duplicate double-counts £21.99 in the user's August expense log, budget
spend, and health score. Not fixing it leaves incorrect financial reporting.

#### Why does this activity or change need to be done under Change Management? Can it be safely automated?

The product has no delete-expense path: the ledger is append-only by design,
and a user-facing delete is tracked as a follow-up ticket. This is a direct
write to prod storage outside the application: a one-off, manually scoped
data change on a live ledger, which needs a documented rollback and an audit
trail. It is a managed manual operation, not a migration.

#### Are there any related, prerequisite changes upon which this CM hinges?

It hinges on this CM item's own creation PR being merged to `main` and the
repo pulled at `/opt/gofin` so `assets/` is available. No code deploy is
required: no service changes ship with this item. The follow-up ticket for
user-facing expense deletion and idempotent creation is tracked separately
and does not block this change.

#### Will this CM be in any way intrusive, and if so, how will you know? What teams, services or functionality will be impacted?

Minimal: one UPDATE on one ledger row, no service restarts, no downtime. The
statement is scoped by primary key id plus user id, so no other row can
match. Impact is detected by re-querying active rows afterward (one remains)
and by checking the app's expense log and dashboard totals.

#### How has this change been tested to verify it's safe for production?

The exact commands were rehearsed against a throwaway
`codenotary/immudb:1.11.0` container: the prod schema was created, two
identical active rows were inserted, the deactivation UPDATE ran
(`Updated rows: 1`), the active-only query then returned one row, and the
inverse UPDATE restored the row to `active`. The rehearsal used the same
client image, flags, and SQL the prod steps use.

## Worst Case Scenario

#### What could happen if everything goes wrong with this change?

The wrong row is deactivated (a row other than the duplicate loses active
status), or the UPDATE matches no row or more than one row, leaving the log
with two entries or hiding an entry it should not.

#### How does this CM attempt to mitigate this risk?

- The UPDATE is scoped by the row's primary key id plus the owner's user id,
  both recorded in preflight, so no other row can match.
- Preflight records the exact two row ids and their created_at order; steps
  re-verify the pre-state before the UPDATE.
- immudb preserves full history: the pre-change state is recoverable via
  time travel (`SELECT ... BEFORE TX`) and the row is restored by the
  inverse UPDATE, both recorded as rollback actions.
- The post-check confirms exactly one active row remains, with the kept id.

## Rollback Procedure

#### What conditions would indicate a need to rollback?

The UPDATE reports anything other than `Updated rows: 1`; the post-check
shows zero or two active rows for the pair; the wrong row was deactivated;
or the app shows incorrect data after the change.

#### In the event of problems, what will you do to return your system to a known good state?

Run the inverse UPDATE to restore the deactivated row:

`UPDATE expenses SET status = 'active' WHERE id = '<DUP_ID>' AND user_id = '<USER_ID>';`

This is a single reversible statement; no restore from backup is needed. If
further investigation is required, immudb time travel
(`SELECT ... BEFORE TX <pre-change-tx>`) returns the ledger state as it was
before the change, using the TX id recorded in steps Activity 2.

#### If this is a software or infrastructure change, has the rollback procedure been verified in a development environment?

The inverse UPDATE was verified in the local rehearsal: flipping the
duplicate back to `active` restored the two-active-row state exactly.
