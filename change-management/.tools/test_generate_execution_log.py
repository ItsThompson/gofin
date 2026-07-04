#!/usr/bin/env python3
"""Unit tests for ``generate_execution_log.py``.

Run directly (no external test runner required):

    python3 change-management/.tools/test_generate_execution_log.py

Sociable tests: they exercise the real parse/render/IO code through the public
functions and assert on observable output, not internal state.
"""

import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import generate_execution_log as gen  # noqa: E402

# A minimal conforming preflight/steps fixture with two matched pairs.
PREFLIGHT = """# Preflight: 001_demo

> Instruction blockquote that must be ignored by the parser.
> Every Activity N must have a Validation N.

# Activity 1: merge the PR

**Description**: merges the change PR to main.

## Checklist:
1. approve the PR
2. click merge

## Rollback Plan:
1. revert the merge commit

# Validation 1: main is green

**Description**: confirm CI passed on main.

## Checklist:
1. CI on main is green

## Rollback Plan:
1. revert if red
"""

STEPS = """# Steps: 001_demo

> Execution and finalization.

# Activity 1: run cleanup.sql

**Description**: executes the destructive cleanup.

## Checklist:
- run the transaction
- verify affected row count

## Rollback Plan:
1. restore from backup

# Validation 1: rows gone

**Description**: confirm the rows are deleted.

## Checklist:
1. select count returns zero

## Rollback Plan:
1. restore from backup
"""


class ParseBlocksTest(unittest.TestCase):
    def test_extracts_activities_and_validations_with_titles(self):
        blocks = gen.parse_blocks(PREFLIGHT)
        self.assertEqual(
            [(b.kind, b.index, b.title) for b in blocks],
            [("Activity", 1, "merge the PR"), ("Validation", 1, "main is green")],
        )

    def test_captures_only_checklist_steps_not_rollback_or_description(self):
        blocks = gen.parse_blocks(PREFLIGHT)
        activity = blocks[0]
        self.assertEqual(activity.steps, ["approve the PR", "click merge"])
        # The rollback item ("revert the merge commit") must not leak in.
        self.assertNotIn("revert the merge commit", activity.steps)

    def test_supports_bulleted_checklist_items(self):
        blocks = gen.parse_blocks(STEPS)
        self.assertEqual(blocks[0].steps, ["run the transaction", "verify affected row count"])

    def test_ignores_blockquote_instruction_lines(self):
        blocks = gen.parse_blocks(PREFLIGHT)
        titles = [b.title for b in blocks]
        self.assertNotIn("Instruction blockquote that must be ignored by the parser.", titles)


class PairBlocksTest(unittest.TestCase):
    def test_pairs_activity_with_matching_validation(self):
        pairs = gen.pair_blocks(gen.parse_blocks(PREFLIGHT))
        self.assertEqual(len(pairs), 1)
        self.assertEqual(pairs[0].activity.index, 1)
        self.assertIsNotNone(pairs[0].validation)
        self.assertEqual(pairs[0].validation.index, 1)

    def test_missing_validation_yields_none_not_dropped_activity(self):
        text = "# Activity 1: lonely\n\n## Checklist:\n1. do it\n"
        pairs = gen.pair_blocks(gen.parse_blocks(text))
        self.assertEqual(len(pairs), 1)
        self.assertIsNone(pairs[0].validation)


class RenderExecutionLogTest(unittest.TestCase):
    def setUp(self):
        self.log = gen.render_execution_log("001_demo", PREFLIGHT, STEPS)

    def test_header_uses_colon_not_em_dash(self):
        self.assertIn("# Execution Log: 001_demo", self.log)
        self.assertNotIn("\u2014", self.log)  # no em-dash anywhere

    def test_header_has_fillable_metadata(self):
        self.assertIn("- Technician: ____________________", self.log)
        self.assertIn("- Date/Time started: ____________________", self.log)
        self.assertIn("- Environment: ____________________", self.log)

    def test_checkbox_per_checklist_step_across_both_sections(self):
        # Preflight activity (2) + validation (1); Steps activity (2) + validation (1).
        self.assertIn("- [ ] approve the PR", self.log)
        self.assertIn("- [ ] click merge", self.log)
        self.assertIn("- [ ] CI on main is green", self.log)
        self.assertIn("- [ ] run the transaction", self.log)
        self.assertIn("- [ ] verify affected row count", self.log)
        self.assertIn("- [ ] select count returns zero", self.log)

    def test_activity_followed_by_validation_and_comments(self):
        preflight_section = self.log.split("## Steps")[0]
        activity_pos = preflight_section.index("### Activity 1: merge the PR")
        validation_pos = preflight_section.index("**Validation 1: main is green**")
        comments_pos = preflight_section.index("> Comments:", validation_pos)
        self.assertLess(activity_pos, validation_pos)
        self.assertLess(validation_pos, comments_pos)

    def test_has_both_named_sections(self):
        self.assertIn("## Preflight", self.log)
        self.assertIn("## Steps", self.log)

    def test_outcome_section_present(self):
        self.assertIn("## Outcome", self.log)
        self.assertIn("- [ ] All activities and validations completed", self.log)
        self.assertIn("> Notes:", self.log)

    def test_completion_footer_names_rename_target_and_statuses(self):
        self.assertIn("COMPLETION REQUIRED", self.log)
        self.assertIn("change-management/001_demo_<status>", self.log)
        for status in ("completed", "completed-off-script", "failed", "aborted"):
            self.assertIn(status, self.log)


class GenerateIoTest(unittest.TestCase):
    def _make_item(self, root: Path) -> Path:
        item = root / "001_demo"
        item.mkdir()
        (item / "preflight.md").write_text(PREFLIGHT, encoding="utf-8")
        (item / "steps.md").write_text(STEPS, encoding="utf-8")
        return item

    def test_writes_execution_log_into_item_folder(self):
        with tempfile.TemporaryDirectory() as tmp:
            item = self._make_item(Path(tmp))
            output = gen.generate(item)
            self.assertEqual(output, item / "execution-log.md")
            self.assertTrue(output.is_file())
            self.assertIn("# Execution Log: 001_demo", output.read_text(encoding="utf-8"))

    def test_refuses_to_overwrite_without_force(self):
        with tempfile.TemporaryDirectory() as tmp:
            item = self._make_item(Path(tmp))
            gen.generate(item)
            with self.assertRaises(FileExistsError):
                gen.generate(item)

    def test_force_overwrites_existing_log(self):
        with tempfile.TemporaryDirectory() as tmp:
            item = self._make_item(Path(tmp))
            output = gen.generate(item)
            output.write_text("stale content", encoding="utf-8")
            gen.generate(item, force=True)
            self.assertIn("# Execution Log: 001_demo", output.read_text(encoding="utf-8"))

    def test_missing_source_raises(self):
        with tempfile.TemporaryDirectory() as tmp:
            item = Path(tmp) / "001_demo"
            item.mkdir()
            (item / "preflight.md").write_text(PREFLIGHT, encoding="utf-8")
            with self.assertRaises(FileNotFoundError):
                gen.generate(item)

    def test_missing_folder_raises(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(FileNotFoundError):
                gen.generate(Path(tmp) / "does-not-exist")


class CliTest(unittest.TestCase):
    def test_main_returns_zero_and_writes(self):
        with tempfile.TemporaryDirectory() as tmp:
            item = Path(tmp) / "001_demo"
            item.mkdir()
            (item / "preflight.md").write_text(PREFLIGHT, encoding="utf-8")
            (item / "steps.md").write_text(STEPS, encoding="utf-8")
            self.assertEqual(gen.main([str(item)]), 0)
            self.assertTrue((item / "execution-log.md").is_file())
            # Second run without --force fails; with --force succeeds.
            self.assertEqual(gen.main([str(item)]), 1)
            self.assertEqual(gen.main([str(item), "--force"]), 0)


class RealTemplateTest(unittest.TestCase):
    """Runs against the actual committed 000-template (acceptance criterion)."""

    def test_generates_fillable_log_from_repo_template(self):
        template = Path(__file__).resolve().parents[1] / "000-template"
        preflight = (template / "preflight.md").read_text(encoding="utf-8")
        steps = (template / "steps.md").read_text(encoding="utf-8")
        log = gen.render_execution_log(template.name, preflight, steps)
        self.assertIn("# Execution Log: 000-template", log)
        self.assertIn("## Preflight", log)
        self.assertIn("## Steps", log)
        self.assertIn("## Outcome", log)
        self.assertIn("COMPLETION REQUIRED", log)
        # The template ships two matched pairs per file; every checklist step
        # becomes a checkbox, so the log must contain checkboxes.
        self.assertGreaterEqual(log.count("- [ ]"), 8)


if __name__ == "__main__":
    unittest.main(verbosity=2)
