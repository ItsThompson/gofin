# Preflight — <NNN>_<kebab-title>

> Everything done *before* the change is executed (merge, deploy, dry-run, backups).
> Every `# Activity N` MUST be followed by a matching `# Validation N` with the same
> index. Each block requires a `**Description**` line, a `## Checklist:` section, and a
> `## Rollback Plan:` section. The validator enforces the pairing and these sections.
> Add or remove pairs as needed; keep the indexes contiguous starting at 1.

# Activity 1: <title>

**Description**: <what this action does and why it happens before execution>

## Checklist:
1. <step>
2. <step>

## Rollback Plan:
1. <how to undo this activity if it must be reversed>

# Validation 1: <title>

**Description**: <how to confirm Activity 1 succeeded>

## Checklist:
1. <observable check that proves success>

## Rollback Plan:
1. <what to do if this validation fails: stop and undo Activity 1>

# Activity 2: <title>

**Description**: <what this action does>

## Checklist:
1. <step>

## Rollback Plan:
1. <how to undo this activity>

# Validation 2: <title>

**Description**: <how to confirm Activity 2 succeeded>

## Checklist:
1. <observable check that proves success>

## Rollback Plan:
1. <what to do if this validation fails>
