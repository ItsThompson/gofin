# Steps: <NNN>_<kebab-title>

> The execution itself, plus any repo housekeeping that finalizes the change (e.g.
> committing the execution log and renaming the folder with a status suffix).
> Every `# Activity N` MUST be followed by a matching `# Validation N` with the same
> index. Each block requires a `**Description**` line, a `## Checklist:` section, and a
> `## Rollback Plan:` section. The validator enforces the pairing and these sections.
> Add or remove pairs as needed; keep the indexes contiguous starting at 1.

# Activity 1: <title>

**Description**: <the change action being performed>

## Checklist:
1. <step>
2. <step>

## Rollback Plan:
1. <how to return the system to a known good state if this action fails>

# Validation 1: <title>

**Description**: <how to confirm Activity 1 succeeded>

## Checklist:
1. <observable check that proves success>

## Rollback Plan:
1. <what to do if this validation fails>

# Activity 2: <title>

**Description**: <finalization action, e.g. commit the execution log and rename the folder with a status suffix>

## Checklist:
1. <step>

## Rollback Plan:
1. <how to undo this finalization action>

# Validation 2: <title>

**Description**: <how to confirm Activity 2 succeeded, e.g. CI including validate-change-management is green>

## Checklist:
1. <observable check that proves success>

## Rollback Plan:
1. <what to do if this validation fails>
