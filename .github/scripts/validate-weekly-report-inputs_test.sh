#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="${SCRIPT_DIR}/validate-weekly-report-inputs.sh"

expect_success() {
  if ! "$SCRIPT" "$@" >/dev/null 2>&1; then
    echo "Expected success: $*" >&2
    exit 1
  fi
}

expect_failure() {
  if "$SCRIPT" "$@" >/dev/null 2>&1; then
    echo "Expected failure: $*" >&2
    exit 1
  fi
}

expect_success "" false
expect_success "2026-06-01" true
expect_success "2024-02-26" false
expect_failure "2026-06-07" false
expect_failure "2026-02-30" false
expect_failure "2026-13-01" false
expect_failure "2026-6-01" false
expect_failure "2026-06-01" yes
expect_failure "2026-06-01"
expect_failure "2026-06-01" false extra

echo "validate-weekly-report-inputs.sh: all tests passed"
