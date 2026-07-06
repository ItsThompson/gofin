#!/usr/bin/env python3
"""Tests for validate_change_management.py.

Fixtures are built in a temp directory at runtime so no malformed items are
committed to the repo. Run from this directory:

    python3 -m unittest test_validate_change_management -v

or via the repo root:

    python3 change-management/.validation/test_validate_change_management.py
"""

from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path

_MODULE_PATH = Path(__file__).resolve().parent / "validate_change_management.py"
_spec = importlib.util.spec_from_file_location("validate_cm", _MODULE_PATH)
assert _spec and _spec.loader
cm = importlib.util.module_from_spec(_spec)
# Register before exec so dataclass decorators can resolve __module__.
sys.modules[_spec.name] = cm
_spec.loader.exec_module(cm)

# The canonical template that ships in the repo; used to prove the validator
# accepts the real reserved template.
_TEMPLATE_DIR = _MODULE_PATH.parent.parent / "000-template"


def _description_md() -> str:
    lines = ["# Description: 001_example", ""]
    for level, title in cm.REQUIRED_DESCRIPTION_HEADINGS:
        lines.append(f"{'#' * level} {title}")
        lines.append("")
        lines.append("<content>")
        lines.append("")
    return "\n".join(lines)


def _av_block(kind: str, index: int) -> str:
    return "\n".join(
        [
            f"# {kind} {index}: title",
            "",
            "**Description**: what this does",
            "",
            "## Checklist:",
            "1. do a thing",
            "",
            "## Rollback Plan:",
            "1. undo the thing",
            "",
        ]
    )


def _av_md(pairs: int = 2) -> str:
    blocks = []
    for index in range(1, pairs + 1):
        blocks.append(_av_block("Activity", index))
        blocks.append(_av_block("Validation", index))
    return "\n".join(blocks)


def _write_valid_item(root: Path, name: str) -> Path:
    item = root / name
    (item / "assets").mkdir(parents=True)
    (item / "description.md").write_text(_description_md(), encoding="utf-8")
    (item / "preflight.md").write_text(_av_md(), encoding="utf-8")
    (item / "steps.md").write_text(_av_md(), encoding="utf-8")
    return item


class ValidateChangeManagementTest(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def _rule(self, name: str) -> str | None:
        violation = cm.validate_item(self.root / name)
        return violation.rule if violation else None

    # --- conforming items -------------------------------------------------

    def test_valid_precompletion_item_passes(self) -> None:
        _write_valid_item(self.root, "001_example-item")
        self.assertIsNone(cm.validate_item(self.root / "001_example-item"))

    def test_shipped_template_passes(self) -> None:
        self.assertIsNone(cm.validate_item(_TEMPLATE_DIR))

    def test_template_name_accepted(self) -> None:
        _write_valid_item(self.root, cm.TEMPLATE_NAME)
        self.assertIsNone(cm.validate_item(self.root / cm.TEMPLATE_NAME))

    def test_completed_item_with_log_passes(self) -> None:
        item = _write_valid_item(self.root, "001_example_completed")
        (item / "execution-log.md").write_text("done", encoding="utf-8")
        self.assertIsNone(cm.validate_item(item))

    def test_all_status_suffixes_accepted(self) -> None:
        for status in cm.STATUS_SUFFIXES:
            item = _write_valid_item(self.root, f"00{1}_x_{status}")
            (item / "execution-log.md").write_text("log", encoding="utf-8")
            self.assertIsNone(cm.validate_item(item), status)

    # --- folder-name violations ------------------------------------------

    def test_bad_padding_fails(self) -> None:
        _write_valid_item(self.root, "1_example")
        self.assertIn("folder name", self._rule("1_example") or "")

    def test_uppercase_title_fails(self) -> None:
        _write_valid_item(self.root, "001_Example")
        self.assertIn("folder name", self._rule("001_Example") or "")

    def test_invalid_status_suffix_fails(self) -> None:
        _write_valid_item(self.root, "001_example_done")
        self.assertIn("folder name", self._rule("001_example_done") or "")

    # --- required-file violations ----------------------------------------

    def test_missing_required_file_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        (item / "steps.md").unlink()
        rule = self._rule("001_example")
        self.assertIn("required file is missing", rule or "")

    # --- description-heading violations ----------------------------------

    def test_missing_heading_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        full = _description_md()
        broken = full.replace("#### What could happen if everything goes "
                              "wrong with this change?", "#### Oops wrong")
        (item / "description.md").write_text(broken, encoding="utf-8")
        self.assertIn("missing required heading", self._rule("001_example") or "")

    # --- activity/validation violations ----------------------------------

    def test_unmatched_activity_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        text = _av_block("Activity", 1) + _av_block("Validation", 1) \
            + _av_block("Activity", 2)
        (item / "preflight.md").write_text(text, encoding="utf-8")
        self.assertIn("Activity 2 has no matching Validation 2",
                      self._rule("001_example") or "")

    def test_stray_validation_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        text = _av_block("Activity", 1) + _av_block("Validation", 1) \
            + _av_block("Validation", 2)
        (item / "preflight.md").write_text(text, encoding="utf-8")
        self.assertIn("Validation 2 has no matching Activity 2",
                      self._rule("001_example") or "")

    def test_validation_only_file_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        text = _av_block("Validation", 1) + _av_block("Validation", 2)
        (item / "steps.md").write_text(text, encoding="utf-8")
        self.assertIn("no '# Activity N' block found",
                      self._rule("001_example") or "")

    def test_block_missing_checklist_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        text = _av_block("Activity", 1).replace("## Checklist:", "## Tasks:") \
            + _av_block("Validation", 1)
        (item / "steps.md").write_text(text, encoding="utf-8")
        self.assertIn("Checklist", self._rule("001_example") or "")

    def test_block_missing_description_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        text = _av_block("Activity", 1).replace("**Description**:",
                                                "Description:") \
            + _av_block("Validation", 1)
        (item / "preflight.md").write_text(text, encoding="utf-8")
        self.assertIn("Description", self._rule("001_example") or "")

    def test_empty_av_file_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        (item / "steps.md").write_text("# Steps\n\njust prose\n",
                                       encoding="utf-8")
        self.assertIn("no '# Activity N'", self._rule("001_example") or "")

    # --- completion-evidence violations ----------------------------------

    def test_completed_without_log_fails(self) -> None:
        _write_valid_item(self.root, "001_example_completed")
        rule = self._rule("001_example_completed") or ""
        self.assertIn("execution-log.md", rule)

    def test_completed_with_empty_log_fails(self) -> None:
        item = _write_valid_item(self.root, "001_example_completed")
        (item / "execution-log.md").write_text("   \n", encoding="utf-8")
        self.assertIn("empty", self._rule("001_example_completed") or "")

    def test_precompletion_without_log_passes(self) -> None:
        _write_valid_item(self.root, "001_example")
        self.assertIsNone(cm.validate_item(self.root / "001_example"))

    # --- assets are not validated ----------------------------------------

    def test_assets_contents_not_validated(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        (item / "assets" / "junk.sql").write_text("### not a heading",
                                                   encoding="utf-8")
        (item / "assets" / "notes.md").write_text("# Activity 9\nbroken",
                                                  encoding="utf-8")
        self.assertIsNone(cm.validate_item(item))

    # --- discovery / runner ----------------------------------------------

    def test_find_items_ignores_dot_dirs_and_files(self) -> None:
        _write_valid_item(self.root, "001_example")
        (self.root / ".validation").mkdir()
        (self.root / ".tools").mkdir()
        (self.root / "README.md").write_text("x", encoding="utf-8")
        names = [p.name for p in cm.find_items(self.root)]
        self.assertEqual(names, ["001_example"])

    def test_main_returns_zero_when_all_conform(self) -> None:
        _write_valid_item(self.root, "001_example")
        _write_valid_item(self.root, cm.TEMPLATE_NAME)
        self.assertEqual(cm.main(["--root", str(self.root)]), 0)

    def test_main_returns_one_on_violation(self) -> None:
        item = _write_valid_item(self.root, "001_example")
        (item / "description.md").unlink()
        self.assertEqual(cm.main(["--root", str(self.root)]), 1)

    def test_main_item_flag_validates_single_item(self) -> None:
        _write_valid_item(self.root, "001_ok")
        bad = _write_valid_item(self.root, "002_bad")
        (bad / "preflight.md").unlink()
        self.assertEqual(
            cm.main(["--root", str(self.root), "--item", "001_ok"]), 0)
        self.assertEqual(
            cm.main(["--root", str(self.root), "--item", "002_bad"]), 1)

    def test_main_unknown_root_returns_two(self) -> None:
        self.assertEqual(cm.main(["--root", str(self.root / "nope")]), 2)


if __name__ == "__main__":
    unittest.main(verbosity=2)
