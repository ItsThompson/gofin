#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="${SCRIPT_DIR}/deploy.sh"

require_literal() {
  local literal=$1
  if ! grep -Fq -- "$literal" "$SCRIPT"; then
    echo "Missing deployment contract: $literal" >&2
    exit 1
  fi
}

bash -n "$SCRIPT"

invalid_sha_output=$(DEPLOY_SHA=not-a-sha "$SCRIPT" 127.0.0.1 2>&1 || true)
if [[ "$invalid_sha_output" != *"exactly 40 lowercase hexadecimal"* ]]; then
  echo "Invalid DEPLOY_SHA did not fail with a strict validation error" >&2
  exit 1
fi

require_literal 'git -C /opt/gofin fetch --no-tags origin main --force'
require_literal 'git -C /opt/gofin reset --hard "$DEPLOY_SHA"'
require_literal 'git -C /opt/gofin reset --hard "$PREVIOUS_SHA"'
require_literal 'target_ref="${image}:sha-${DEPLOY_SHA}"'
require_literal 'restore_tmp="$(mktemp "${env_file}.rollback.XXXXXX")"'
require_literal 'current_ref="$(docker inspect --format '\''{{.Config.Image}}'\'' "$container")"'
require_literal 'if [[ ! "$current_ref" =~ :sha-[0-9a-f]{40}$ ]]; then'
require_literal 'current_ref_id="$(docker image inspect --format '\''{{.Id}}'\'' "$current_ref")"'
require_literal 'has("expense-service")'
require_literal '[[ "$service" == "reporting" ]] && continue'
require_literal 'docker pull "$old_ref"'
require_literal '--no-deps --remove-orphans'
require_literal 'marker_tmp="$(mktemp "${marker}.tmp.XXXXXX")"'
require_literal 'mv -f "$marker_tmp" "$marker"'

pull_match=$(grep -m1 -n 'docker pull "$target_ref"' "$SCRIPT" || true)
old_pull_match=$(grep -m1 -n 'docker pull "$old_ref"' "$SCRIPT" || true)
env_rewrite_match=$(grep -m1 -n 'mv -f "$env_tmp" "$env_file"' "$SCRIPT" || true)
pull_line=${pull_match%%:*}
old_pull_line=${old_pull_match%%:*}
env_rewrite_line=${env_rewrite_match%%:*}
if [[ -z "$pull_line" || -z "$old_pull_line" || -z "$env_rewrite_line" || "$pull_line" -ge "$env_rewrite_line" || "$old_pull_line" -ge "$env_rewrite_line" ]]; then
  echo "Image preflight must complete before .env mutation" >&2
  exit 1
fi
if grep -Fq 'ps -a --format json 2>/dev/null || true' "$SCRIPT"; then
  echo "Compose health-query failures must not be suppressed" >&2
  exit 1
fi

previous_reset_match=$(grep -m1 -n 'git -C /opt/gofin reset --hard "$PREVIOUS_SHA"' "$SCRIPT" || true)
rollback_start_match=$(grep -m1 -n 'rollback_args=(docker compose' "$SCRIPT" || true)
previous_reset_line=${previous_reset_match%%:*}
rollback_start_line=${rollback_start_match%%:*}
if [[ -z "$previous_reset_line" || -z "$rollback_start_line" || "$previous_reset_line" -ge "$rollback_start_line" ]]; then
  echo "Rollback must reset the checkout before restarting containers" >&2
  exit 1
fi
if grep -Fq 'rm -f "$marker"' "$SCRIPT"; then
  echo "Rollback must preserve the prior deployment marker" >&2
  exit 1
fi

if grep -Fq 'docker tag' "$SCRIPT"; then
  echo "Rollback must not retag immutable images as latest" >&2
  exit 1
fi
if grep -Fq 'docker compose pull' "$SCRIPT"; then
  echo "Deployment must pull explicit immutable image references" >&2
  exit 1
fi

echo "deploy.sh: all tests passed"
