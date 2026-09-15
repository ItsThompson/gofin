#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="${SCRIPT_DIR}/verify-reporting-compose.sh"
FAKE_DOCKER="${SCRIPT_DIR}/verify-reporting-compose_fake-docker.sh"
VALID_FIXTURE="${SCRIPT_DIR}/verify-reporting-compose_testdata.json"
BAD_FIXTURE="${SCRIPT_DIR}/verify-reporting-compose_bad.json"

# The helper resolves docker by name. Put the checked-in fake at the front of PATH.
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT
cp "$FAKE_DOCKER" "$tmp_dir/docker"
chmod +x "$tmp_dir/docker"

if ! COMPOSE_FIXTURE="$VALID_FIXTURE" PATH="$tmp_dir:$PATH" "$SCRIPT" >"$tmp_dir/output" 2>"$tmp_dir/errors"; then
  cat "$tmp_dir/errors" >&2
  exit 1
fi

grep -q '"reporting"' "$tmp_dir/output"
grep -q '"environment_names"' "$tmp_dir/output"
grep -q '"arg_names"' "$tmp_dir/output"
if grep -q 'do-not-print\|also-do-not-print\|CANARY-SENTRY-DSN' "$tmp_dir/output"; then
  echo "Compose interpolated value leaked in success output" >&2
  exit 1
fi
if grep -q 'do-not-print\|also-do-not-print\|CANARY-SENTRY-DSN' "$tmp_dir/errors"; then
  echo "Compose interpolated value leaked in success stderr" >&2
  exit 1
fi

if COMPOSE_FIXTURE="$BAD_FIXTURE" PATH="$tmp_dir:$PATH" "$SCRIPT" >"$tmp_dir/bad-output" 2>"$tmp_dir/bad-errors"; then
  echo "Expected DB credentials to fail validation" >&2
  exit 1
fi
if grep -q 'postgres://secret\|CANARY-SENTRY-DSN' "$tmp_dir/bad-output"; then
  echo "Compose interpolated value leaked in failure output" >&2
  exit 1
fi
if grep -q 'postgres://secret\|CANARY-SENTRY-DSN' "$tmp_dir/bad-errors"; then
  echo "Compose interpolated value leaked in failure stderr" >&2
  exit 1
fi
grep -q 'isolated Compose contract' "$tmp_dir/bad-errors"

echo "verify-reporting-compose.sh: all tests passed"
