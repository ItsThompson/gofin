#!/usr/bin/env python3
"""Generate a fillable ``execution-log.md`` for a change-management item.

Reads a change-management item's ``preflight.md`` and ``steps.md`` (the paired
``# Activity N`` / ``# Validation N`` structure defined in spec section 6) and
renders an ``execution-log.md``: a per-activity/validation checklist the
technician ticks off during execution and commits as the item's audit record.

Parse-and-render only, one command, no persistent state.

    usage: generate_execution_log.py <item-folder> [--force]
      writes <item-folder>/execution-log.md (refuses to overwrite unless --force)
"""

from __future__ import annotations

import argparse
import os
import re
import sys
import tempfile
from dataclasses import dataclass, field
from pathlib import Path

# A block heading, e.g. "# Activity 1: merge the PR" or "# Validation 2: CI green".
_HEADING_RE = re.compile(r"^#\s+(Activity|Validation)\s+(\d+)\s*:\s*(.*)$")
# A checklist item: numbered ("1. foo", "2) bar") or bulleted ("- foo", "* bar").
_LIST_ITEM_RE = re.compile(r"^\s*(?:\d+[.)]|[-*+])\s+(.*\S)\s*$")
# The literal folder name reserved for the copyable template.
_TEMPLATE_NAME = "000-template"

VALID_STATUSES = ("completed", "completed-off-script", "failed", "aborted")


@dataclass
class Block:
    """A parsed ``Activity`` or ``Validation`` block with its checklist steps."""

    kind: str  # "Activity" or "Validation"
    index: int
    title: str
    steps: list[str] = field(default_factory=list)


@dataclass
class Pair:
    """An ``Activity N`` and its matching ``Validation N`` (if present)."""

    index: int
    activity: Block
    validation: Block | None


def parse_blocks(text: str) -> list[Block]:
    """Parse ``Activity``/``Validation`` blocks and their checklist steps.

    Only the ``## Checklist:`` section of each block contributes steps; the
    ``**Description**`` line and ``## Rollback Plan:`` section are intentionally
    ignored (they are guidance, not runtime checks). Blockquote instruction
    lines (``> ...``) never match a heading, so template preamble is skipped.
    """
    blocks: list[Block] = []
    current: Block | None = None
    in_checklist = False

    for line in text.splitlines():
        heading = _HEADING_RE.match(line)
        if heading:
            current = Block(
                kind=heading.group(1),
                index=int(heading.group(2)),
                title=heading.group(3).strip(),
            )
            blocks.append(current)
            in_checklist = False
            continue

        if current is None:
            continue

        stripped = line.strip()
        if stripped.startswith("## "):
            # Any level-2 heading toggles checklist capture on only for Checklist:.
            in_checklist = stripped.rstrip().lower().startswith("## checklist")
            continue

        if in_checklist:
            item = _LIST_ITEM_RE.match(line)
            if item:
                current.steps.append(item.group(1).strip())

    return blocks


def pair_blocks(blocks: list[Block]) -> list[Pair]:
    """Group blocks into ordered ``Activity``/``Validation`` pairs by index.

    Iterates activities in the order they appear and attaches the matching
    validation by index. A missing validation yields a ``Pair`` with
    ``validation=None`` rather than dropping the activity, so the generated log
    still surfaces the gap for the technician (the validator, #10, is what
    enforces the pairing rule).
    """
    validations = {b.index: b for b in blocks if b.kind == "Validation"}
    pairs: list[Pair] = []
    for block in blocks:
        if block.kind != "Activity":
            continue
        pairs.append(Pair(index=block.index, activity=block, validation=validations.get(block.index)))
    return pairs


def _render_steps(steps: list[str]) -> list[str]:
    """Render checklist steps as ``- [ ]`` checkboxes (placeholder if empty)."""
    if not steps:
        return ["- [ ] (no checklist steps defined)"]
    return [f"- [ ] {step}" for step in steps]


def _render_pairs(pairs: list[Pair]) -> list[str]:
    lines: list[str] = []
    for pair in pairs:
        activity = pair.activity
        lines.append(f"### Activity {activity.index}: {activity.title}")
        lines.extend(_render_steps(activity.steps))

        validation = pair.validation
        if validation is not None:
            lines.append(f"**Validation {validation.index}: {validation.title}**")
            lines.extend(_render_steps(validation.steps))
        else:
            lines.append(f"**Validation {pair.index}: (missing: add a Validation {pair.index} block)**")
            lines.append("- [ ] (no matching validation defined)")

        lines.append("> Comments: (off-script actions, log output, anomalies)")
        lines.append("")
    return lines


def render_execution_log(item_name: str, preflight_text: str, steps_text: str) -> str:
    """Render the full ``execution-log.md`` body for an item.

    Pure: takes the item name and the raw ``preflight.md`` / ``steps.md`` text
    and returns the markdown. The header uses a colon rather than an em-dash to
    comply with the repo-wide prohibition on em-dashes in tracked files.
    """
    lines: list[str] = [
        f"# Execution Log: {item_name}",
        "",
        "- Technician: ____________________",
        "- Date/Time started: ____________________",
        "- Environment: ____________________",
        "",
        "## Preflight",
    ]
    lines.extend(_render_pairs(pair_blocks(parse_blocks(preflight_text))))

    lines.append("## Steps")
    lines.extend(_render_pairs(pair_blocks(parse_blocks(steps_text))))

    lines.extend(
        [
            "## Outcome",
            "- [ ] All activities and validations completed",
            "> Notes:",
            "",
            "---",
            "COMPLETION REQUIRED: rename this item's folder to",
            f"  change-management/{item_name}_<status>",
            f"where <status> is one of: {' | '.join(VALID_STATUSES)}",
            "Open a PR with the rename and this filled-in execution log. CI will re-validate.",
            "",
        ]
    )
    return "\n".join(lines)


def _write_atomic(destination: Path, content: str) -> None:
    """Write ``content`` to ``destination`` via a temp file + atomic rename."""
    fd, tmp_path = tempfile.mkstemp(dir=str(destination.parent), prefix=".execution-log.", suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as tmp_file:
            tmp_file.write(content)
        os.replace(tmp_path, destination)
    except BaseException:
        # Leave the destination untouched if anything fails mid-write.
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)
        raise


def generate(item_folder: Path, force: bool = False) -> Path:
    """Read an item's sources and write ``execution-log.md``. Returns its path.

    Raises ``FileNotFoundError`` if the folder or a required source is missing,
    and ``FileExistsError`` if the log already exists and ``force`` is False.
    """
    if not item_folder.is_dir():
        raise FileNotFoundError(f"item folder not found: {item_folder}")

    preflight = item_folder / "preflight.md"
    steps = item_folder / "steps.md"
    for required in (preflight, steps):
        if not required.is_file():
            raise FileNotFoundError(f"required source not found: {required}")

    output = item_folder / "execution-log.md"
    if output.exists() and not force:
        raise FileExistsError(f"{output} already exists (pass --force to overwrite)")

    content = render_execution_log(
        item_name=item_folder.name,
        preflight_text=preflight.read_text(encoding="utf-8"),
        steps_text=steps.read_text(encoding="utf-8"),
    )
    _write_atomic(output, content)
    return output


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="generate_execution_log.py",
        description="Generate a fillable execution-log.md from an item's preflight.md + steps.md.",
    )
    parser.add_argument("item_folder", help="path to the change-management item folder")
    parser.add_argument(
        "--force",
        action="store_true",
        help="overwrite an existing execution-log.md",
    )
    args = parser.parse_args(argv)

    try:
        output = generate(Path(args.item_folder), force=args.force)
    except (FileNotFoundError, FileExistsError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1

    print(f"wrote {output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
