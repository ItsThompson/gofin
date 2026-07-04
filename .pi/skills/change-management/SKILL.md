---
name: change-management
description: >
  GoFin change-management framework. Activate when creating, executing, or
  reviewing an operational/destructive change under change-management/ (e.g.
  one-off data cleanups, manual prod procedures). Covers folder naming, the
  description/preflight/steps files, the activity/validation pairing rule, the
  validator and execution-log generator, and the completion (status suffix +
  execution log) workflow.
---

# Change Management

The `change-management/` directory (repo root) is a durable, auditable framework
for delivering operational changes that are NOT ordinary code migrations:
destructive data operations and manual production procedures. Each change is a
numbered, immutable-ish record that accrues history, modeled on how ADRs are
organized.

This skill is the how-to. The copyable file formats live in
`change-management/000-template/`; read those files rather than trusting a
paraphrase here, and copy them to start a new item.

## When to Use Change Management

Use a CM item when the change is a **managed manual operation**, for example:

- A destructive one-off data operation (bulk `DELETE`/`UPDATE` against prod).
- A manual production procedure with a blast radius that needs a documented
  rollback and an auditable record of who ran it and what happened.
- Anything where "just run the migration" is unsafe because the action cannot
  be trivially reversed and needs a human executing checklists with validations.

Use a **normal migration** (e.g. `services/dbmigrate`) instead when the change
is idempotent, safely automatable, and reversible through ordinary
forward/backward migration tooling. If a golang-migrate migration expresses the
change safely, it does not belong under `change-management/`.

Rule of thumb: if you need a rollback plan written per step and a technician
signing off on validations, it is a CM item. If CI can apply and revert it
mechanically, it is a migration.

## Directory Layout

```
change-management/
├── .validation/
│   └── validate_change_management.py   # template + status validator (CI + local)
├── .tools/
│   └── generate_execution_log.py       # builds execution-log.md from preflight.md + steps.md
├── 000-template/                       # reserved: the canonical template to copy
│   ├── description.md
│   ├── preflight.md
│   ├── steps.md
│   └── assets/                         # optional supporting files (.gitkeep keeps it tracked)
└── <NNN>_<kebab-title>[_<status>]/     # one folder per change item
    ├── description.md
    ├── preflight.md
    ├── steps.md
    ├── execution-log.md                # present once executed (completion PR)
    └── assets/                         # optional (scripts, images, SQL, ...)
```

- `.validation/` and `.tools/` are dot-prefixed and are NOT treated as items by
  the validator.
- `000-template` is the reserved literal (hyphen). It is not an executable item;
  the validator checks its file structure but skips status/log checks.

## Naming Convention

| Element | Rule |
|---------|------|
| Item folder (pre-completion) | `<NNN>_<kebab-title>`; `NNN` = zero-padded sequential id, `<kebab-title>` = lowercase kebab-case. Regex: `^\d{3}_[a-z0-9]+(-[a-z0-9]+)*$` |
| Item folder (completed) | `<NNN>_<kebab-title>_<status>` where `status ∈ {completed, completed-off-script, failed, aborted}` |
| Template | The reserved literal `000-template` (hyphen) |
| Tooling dirs | `.validation/` and `.tools/` (dot-prefixed, ignored as items) |

Ids are assigned sequentially by the author: pick the next unused number. The
template is `000`; the first real item is `001`. The per-folder regex does not
auto-enforce uniqueness, so confirm your id is not already taken before
creating.

## Creating an Item

1. Pick the next unused sequential id (`NNN`) and a lowercase kebab-case title.
2. Copy the template:

   ```
   cp -r change-management/000-template change-management/<NNN>_<kebab-title>
   ```

3. Fill in `description.md`, `preflight.md`, and `steps.md`. Do not add, remove,
   or rename the enforced headings. Replace every `<...>` placeholder.
4. Put any supporting files (SQL, scripts, images) under the item's `assets/`.
   `assets/` contents are not validated.
5. Do NOT create `execution-log.md` yet and do NOT add a status suffix: those
   arrive at completion.
6. Run the validator locally (below) and open the item-creation PR.

## The Three Required Files

Every item folder must contain `description.md`, `preflight.md`, and `steps.md`.
The authoritative formats are the files in `change-management/000-template/`:
read and copy those. Their roles and enforced shapes:

### `description.md`: rationale and risk assessment

A fixed section structure the validator enforces. It must contain these `##`
sections, each with its `####` prompts (see `000-template/description.md` for
the full prompt wording):

- `## Change Event` (5 prompts: purpose, what's required to execute, expected
  end state, assumptions about system state, rollout date/time + duration)
- `## Impact / Risk Assessment` (5 prompts: why necessary, why under CM / can it
  be automated, prerequisite changes, intrusiveness + impacted teams/services,
  how it was tested for prod safety)
- `## Worst Case Scenario` (2 prompts: worst realistic failure, how this CM
  mitigates it)
- `## Rollback Procedure` (3 prompts: rollback triggers, actions to reach a
  known-good state, whether rollback was rehearsed in a dev environment)

Do not add, remove, or rename these headings.

### `preflight.md`: everything before execution

Merge, deploy, dry-run, backups: the work done before the change is executed.

### `steps.md`: the execution itself

The change action plus any repo housekeeping that finalizes the change
(committing the execution log, renaming the folder with a status suffix).

Both `preflight.md` and `steps.md` use the paired activity/validation structure
below.

## Activity / Validation Pairing Rule

This is the framework's core discipline: **no action ships without a defined way
to confirm it and a rollback if the confirmation fails.**

- Every `# Activity N` MUST be immediately followed by a matching
  `# Validation N` with the same index.
- Keep indexes contiguous, starting at 1.
- Each block (Activity and Validation alike) MUST contain:
  - a `**Description**` line,
  - a `## Checklist:` section,
  - a `## Rollback Plan:` section.

If an `Activity N` has no matching `Validation N`, the validator fails. See
`change-management/000-template/preflight.md` and `steps.md` for the exact
block layout; add or remove pairs as needed.

## Status Lifecycle

```
   (author creates item)        (technician executes)         (completion PR)
000-template ─copy─▶ NNN_title ───────────────────────▶ NNN_title_<status>
                     no status suffix                    execution-log.md required
                     execution-log.md not required
```

| Status | Meaning |
|--------|---------|
| `completed` | All activities executed as written; all validations passed. |
| `completed-off-script` | Completed, but the technician deviated from the written steps; deviations documented in `execution-log.md`. |
| `failed` | Execution could not be completed successfully; state and follow-up documented. |
| `aborted` | Execution was stopped before completion (e.g. a preflight validation failed) and the system was returned to a known-good state. |

## Running the Validator

`change-management/.validation/validate_change_management.py` is a Python 3 CLI
that runs locally and in CI. It validates folder names, required files,
`description.md` headings, the activity/validation pairing, and completion
evidence.

```
# validate every item
python change-management/.validation/validate_change_management.py

# validate a single item
python change-management/.validation/validate_change_management.py --item <NNN>_<kebab-title>

# non-default root (rarely needed)
python change-management/.validation/validate_change_management.py --root change-management
```

- Exit `0`: all validated items conform.
- Exit `1`: a violation; it prints the item, file, and failing rule.

Run it before opening either PR (item creation and completion). CI runs the same
command on every push/PR via the `validate-change-management` job in
`.github/workflows/ci.yml`, so a violation blocks merge.

## Generating the Execution Log

`change-management/.tools/generate_execution_log.py` reads an item's
`preflight.md` and `steps.md` and writes `execution-log.md` into the item
folder. The generated log is the checklist the technician fills in during
execution and commits as evidence.

```
# generate the log for an item
python change-management/.tools/generate_execution_log.py change-management/<NNN>_<kebab-title>

# overwrite an existing log
python change-management/.tools/generate_execution_log.py change-management/<NNN>_<kebab-title> --force
```

It refuses to overwrite an existing `execution-log.md` unless `--force` is
passed. Generate the log at execution time, not at item creation.

## Completion Workflow

When the change has been executed:

1. **Generate the execution log** if you have not already
   (`generate_execution_log.py <item-folder>`).
2. **Fill it in** during execution: technician, date/time, environment, each
   activity/validation checkbox, and Comments for any off-script actions, log
   output, or anomalies. Record the outcome.
3. **Rename the folder** with the outcome status suffix:

   ```
   git mv change-management/<NNN>_<kebab-title> change-management/<NNN>_<kebab-title>_<status>
   ```

   where `<status>` is one of `completed | completed-off-script | failed |
   aborted`.
4. **Open the completion PR** with the rename and the filled-in
   `execution-log.md`. CI re-validates: a status-suffixed folder must contain a
   non-empty `execution-log.md`, or the `validate-change-management` job fails
   and merge is blocked.

## Common Failures

| Case | Result |
|------|--------|
| An `Activity N` has no matching `Validation N` | Validator fails: unmatched activity. |
| Folder renamed with an invalid status suffix | Folder-name regex fails. |
| Completed folder missing `execution-log.md` | Validator fails (status suffix requires the log). |
| Wrong id padding (e.g. `1_foo`) | Folder-name regex fails (`\d{3}` required). |
| Empty `assets/` | Allowed; keep it tracked with `.gitkeep`. |
| Two items reuse an id | Not auto-enforced; assign ids sequentially and check first. |
