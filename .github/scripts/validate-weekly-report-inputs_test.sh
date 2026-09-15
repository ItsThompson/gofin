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

shift_date() {
  local date_value=$1
  local offset_days=$2
  local shifted
  if shifted=$(date -u -d "$date_value ${offset_days} days" +%F 2>/dev/null); then
    printf '%s\n' "$shifted"
    return
  fi
  local adjustment
  if (( offset_days >= 0 )); then
    adjustment="+${offset_days}d"
  else
    adjustment="${offset_days}d"
  fi
  date -u -j -f "%Y-%m-%d" -v"$adjustment" "$date_value" +%F
}

current_date=$(date -u +%F)
weekday=$(date -u +%u)
days_since_monday=$((weekday - 1))
current_monday=$(shift_date "$current_date" "-$days_since_monday")
previous_monday=$(shift_date "$current_monday" -7)
future_monday=$(shift_date "$current_monday" 7)

expect_success "" false
expect_success "$previous_monday" true
expect_failure "$current_monday" false
expect_failure "$future_monday" false
expect_failure "2026-06-07" false
expect_failure "2026-02-30" false
expect_failure "2026-13-01" false
expect_failure "2026-6-01" false
expect_failure "$previous_monday" yes
expect_failure "$previous_monday"
expect_failure "$previous_monday" false extra

echo "validate-weekly-report-inputs.sh: all tests passed"
