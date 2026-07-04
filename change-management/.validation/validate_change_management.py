#!/usr/bin/env python3
"""Validate change-management item folders against the framework rules (see
`change-management/` section 6 of the Admin Operator Refactor spec).

Runs in CI and on a developer machine (never on the prod VPS). Validates a
single item (``--item <folder>``) or every item under the root (default).

For each item the following checks run, in order, and the FIRST violation for
that item is reported (item, file, failing rule):

  1. Folder name    - pre-completion regex, or completed regex with a valid
                      status suffix; ``000-template`` is the reserved literal.
  2. Required files - ``description.md``, ``preflight.md``, ``steps.md``.
  3. Description     - all required ``##``/``####`` headings are present.
  4. Activity/Val.   - ``preflight.md`` and ``steps.md`` pair every
                      ``# Activity N`` with a ``# Validation N``; each block
                      carries a ``**Description**`` line, ``## Checklist:``,
                      and ``## Rollback Plan:``.
  5. Completion      - a folder with a status suffix requires a non-empty
                      ``execution-log.md``; a folder without one does not.

``assets/`` contents are never validated (arbitrary supporting files).

Exit codes:
  0  every validated item conforms
  1  at least one item has a violation (each is printed to stderr)
  2  usage error (root or item path does not exist)
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

TEMPLATE_NAME = "000-template"
REQUIRED_FILES = ("description.md", "preflight.md", "steps.md")
EXECUTION_LOG = "execution-log.md"
STATUS_SUFFIXES = ("completed-off-script", "completed", "failed", "aborted")

# Folder naming (section 6). Titles are lowercase kebab-case and never contain
# an underscore, so the underscore before an optional status suffix is
# unambiguous.
_TITLE = r"[a-z0-9]+(?:-[a-z0-9]+)*"
_STATUS = "|".join(STATUS_SUFFIXES)
PRE_COMPLETION_RE = re.compile(rf"^\d{{3}}_{_TITLE}$")
COMPLETED_RE = re.compile(rf"^\d{{3}}_{_TITLE}_(?:{_STATUS})$")

# The fixed heading structure the `description.md` must carry (level, text).
REQUIRED_DESCRIPTION_HEADINGS: tuple[tuple[int, str], ...] = (
    (2, "Change Event"),
    (4, "What is the purpose of this activity or change?"),
    (4, "What will be required to execute this change?"),
    (4, "What is the expected end state of the system after this change?"),
    (4, "What assumptions, if any, are being made about the state of the "
        "system at the time of this change?"),
    (4, "Rollout Date/Time(s) and Duration"),
    (2, "Impact / Risk Assessment"),
    (4, "Why is it necessary? What is the impact of not making this change?"),
    (4, "Why does this activity or change need to be done under Change "
        "Management? Can it be safely automated?"),
    (4, "Are there any related, prerequisite changes upon which this CM "
        "hinges?"),
    (4, "Will this CM be in any way intrusive, and if so, how will you know? "
        "What teams, services or functionality will be impacted?"),
    (4, "How has this change been tested to verify it's safe for production?"),
    (2, "Worst Case Scenario"),
    (4, "What could happen if everything goes wrong with this change?"),
    (4, "How does this CM attempt to mitigate this risk?"),
    (2, "Rollback Procedure"),
    (4, "What conditions would indicate a need to rollback?"),
    (4, "In the event of problems, what will you do to return your system to "
        "a known good state?"),
    (4, "If this is a software or infrastructure change, has the rollback "
        "procedure been verified in a development environment?"),
)

_HEADING_RE = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
_FENCE_RE = re.compile(r"^\s*(```|~~~)")
_AV_HEADER_RE = re.compile(r"^#\s+(Activity|Validation)\s+(\d+)\b")
_DESCRIPTION_LINE_RE = re.compile(r"^\*\*Description\*\*")
_CHECKLIST_RE = re.compile(r"^##\s+Checklist:")
_ROLLBACK_RE = re.compile(r"^##\s+Rollback Plan:")


@dataclass(frozen=True)
class Violation:
    """A single conformance failure for one item."""

    item: str
    file: str  # "" for a folder-level (naming) violation
    rule: str

    def __str__(self) -> str:
        where = f"{self.item}/{self.file}" if self.file else self.item
        return f"FAIL [{where}]: {self.rule}"


@dataclass
class Block:
    """One `# Activity N` / `# Validation N` block and its body lines."""

    kind: str
    index: int
    lines: list[str]


def is_status_suffixed(name: str) -> bool:
    """True when the folder name carries a valid completion status suffix."""
    return name != TEMPLATE_NAME and bool(COMPLETED_RE.match(name))


def check_folder_name(name: str) -> str | None:
    if name == TEMPLATE_NAME:
        return None
    if PRE_COMPLETION_RE.match(name) or COMPLETED_RE.match(name):
        return None
    return (
        f"folder name does not match '<NNN>_<kebab-title>' or "
        f"'<NNN>_<kebab-title>_<status>' "
        f"(status one of: {', '.join(sorted(STATUS_SUFFIXES))})"
    )


def _parse_headings(text: str) -> set[tuple[int, str]]:
    """Return the set of (level, text) ATX headings, ignoring fenced code."""
    headings: set[tuple[int, str]] = set()
    in_fence = False
    for line in text.splitlines():
        if _FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        match = _HEADING_RE.match(line)
        if match:
            headings.add((len(match.group(1)), match.group(2).strip()))
    return headings


def check_description_headings(text: str) -> str | None:
    present = _parse_headings(text)
    for level, title in REQUIRED_DESCRIPTION_HEADINGS:
        if (level, title) not in present:
            return f"missing required heading: {'#' * level} {title}"
    return None


def _parse_av_blocks(text: str) -> list[Block]:
    """Split the file into `# Activity N` / `# Validation N` blocks."""
    blocks: list[Block] = []
    current: Block | None = None
    in_fence = False
    for line in text.splitlines():
        if _FENCE_RE.match(line):
            in_fence = not in_fence
            if current is not None:
                current.lines.append(line)
            continue
        if not in_fence:
            header = _AV_HEADER_RE.match(line)
            if header:
                current = Block(header.group(1), int(header.group(2)), [])
                blocks.append(current)
                continue
        if current is not None:
            current.lines.append(line)
    return blocks


def _check_block_sections(block: Block) -> str | None:
    body = block.lines
    if not any(_DESCRIPTION_LINE_RE.match(line) for line in body):
        return "missing '**Description**' line"
    if not any(_CHECKLIST_RE.match(line) for line in body):
        return "missing '## Checklist:' section"
    if not any(_ROLLBACK_RE.match(line) for line in body):
        return "missing '## Rollback Plan:' section"
    return None


def check_activity_validation(text: str) -> str | None:
    blocks = _parse_av_blocks(text)
    if not blocks:
        return "no '# Activity N' / '# Validation N' blocks found"

    activities: set[int] = set()
    validations: set[int] = set()
    for block in blocks:
        section_error = _check_block_sections(block)
        if section_error:
            return f"{block.kind} {block.index}: {section_error}"
        target = activities if block.kind == "Activity" else validations
        target.add(block.index)

    if not activities:
        return "no '# Activity N' block found"
    for index in sorted(activities):
        if index not in validations:
            return f"Activity {index} has no matching Validation {index}"
    for index in sorted(validations):
        if index not in activities:
            return f"Validation {index} has no matching Activity {index}"
    return None


def check_completion_evidence(item_path: Path) -> str | None:
    log = item_path / EXECUTION_LOG
    if not log.is_file():
        return (
            "folder has a status suffix but is missing execution-log.md "
            "(completion evidence is required)"
        )
    if not log.read_text(encoding="utf-8").strip():
        return "execution-log.md is present but empty"
    return None


def validate_item(item_path: Path) -> Violation | None:
    """Validate one item folder; return the first violation, or None."""
    name = item_path.name

    name_error = check_folder_name(name)
    if name_error:
        return Violation(name, "", name_error)

    for required in REQUIRED_FILES:
        if not (item_path / required).is_file():
            return Violation(name, required, "required file is missing")

    description = (item_path / "description.md").read_text(encoding="utf-8")
    description_error = check_description_headings(description)
    if description_error:
        return Violation(name, "description.md", description_error)

    for md_file in ("preflight.md", "steps.md"):
        text = (item_path / md_file).read_text(encoding="utf-8")
        av_error = check_activity_validation(text)
        if av_error:
            return Violation(name, md_file, av_error)

    if is_status_suffixed(name):
        completion_error = check_completion_evidence(item_path)
        if completion_error:
            return Violation(name, EXECUTION_LOG, completion_error)

    return None


def find_items(root: Path) -> list[Path]:
    """All item folders under root (dot-prefixed dirs like .validation and
    .tools are ignored, as are regular files)."""
    return [
        entry
        for entry in sorted(root.iterdir())
        if entry.is_dir() and not entry.name.startswith(".")
    ]


def _default_root() -> Path:
    # The script lives at change-management/.validation/<this file>.
    return Path(__file__).resolve().parent.parent


def _resolve_root(root_arg: str | None) -> Path:
    return Path(root_arg) if root_arg else _default_root()


def _resolve_item(root: Path, item_arg: str) -> Path | None:
    candidate = Path(item_arg)
    if candidate.is_dir():
        return candidate
    joined = root / item_arg
    return joined if joined.is_dir() else None


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Validate change-management item folders.",
    )
    parser.add_argument(
        "--item",
        help="validate a single item folder (name under --root, or a path)",
    )
    parser.add_argument(
        "--root",
        help="root directory holding the items (default: the "
        "change-management/ directory containing this script)",
    )
    args = parser.parse_args(argv)

    root = _resolve_root(args.root)
    if not root.is_dir():
        print(f"error: root '{root}' is not a directory", file=sys.stderr)
        return 2

    if args.item:
        item_path = _resolve_item(root, args.item)
        if item_path is None:
            print(f"error: item '{args.item}' not found under {root}",
                  file=sys.stderr)
            return 2
        items = [item_path]
    else:
        items = find_items(root)

    violations = [v for v in (validate_item(item) for item in items) if v]

    if violations:
        for violation in violations:
            print(violation, file=sys.stderr)
        return 1

    print(f"OK: {len(items)} change-management item(s) conform.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
