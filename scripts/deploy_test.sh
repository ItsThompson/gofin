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

require_literal 'git -C "$DEPLOY_ROOT" fetch --no-tags origin main --force'
require_literal 'git -C "$DEPLOY_ROOT" reset --hard "$DEPLOY_SHA"'
require_literal 'checkout_restore_sha="$(git -C "$DEPLOY_ROOT" rev-parse HEAD 2>/dev/null || true)"'
require_literal 'git -C "$DEPLOY_ROOT" cat-file -e "${PREVIOUS_SHA}^{commit}"'
require_literal 'git -C "$DEPLOY_ROOT" reset --hard "$PREVIOUS_SHA"'
require_literal 'checkout does not match the deployed marker'
require_literal 'target_ref="${image}:sha-${DEPLOY_SHA}"'
require_literal 'restore_tmp="$(mktemp "${env_file}.rollback.XXXXXX")"'
require_literal 'current_ref="$(docker inspect --format '\''{{.Config.Image}}'\'' "$container")"'
require_literal 'if [[ ! "$current_ref" =~ :sha-[0-9a-f]{40}$ ]]; then'
require_literal 'previous_ref="${current_ref%:latest}:sha-${PREVIOUS_SHA}"'
require_literal 'mutable image does not match the deployed marker'
require_literal 'current_ref_id="$(docker image inspect --format '\''{{.Id}}'\'' "$current_ref")"'
require_literal 'has("expense-service")'
require_literal '[[ "$service" == "reporting" ]] && continue'
require_literal 'docker pull "$old_ref"'
require_literal '--no-deps --remove-orphans'
require_literal 'marker_tmp="$(mktemp "${marker}.tmp.XXXXXX")"'
require_literal 'mv -f "$marker_tmp" "$marker"'
require_literal 'trap finish EXIT'
require_literal 'restore_marker'
require_literal 'FATAL: rollback failed; original deployment error was'
require_literal 'docker compose exec -T auth-service /service seed-admin'
require_literal 'transaction_committed=true'

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

previous_reset_match=$(grep -m1 -n 'git -C "$DEPLOY_ROOT" reset --hard "$PREVIOUS_SHA"' "$SCRIPT" || true)
rollback_start_match=$(grep -m1 -n 'rollback_args=(docker compose' "$SCRIPT" || true)
previous_reset_line=${previous_reset_match%%:*}
rollback_start_line=${rollback_start_match%%:*}
if [[ -z "$previous_reset_line" || -z "$rollback_start_line" || "$previous_reset_line" -ge "$rollback_start_line" ]]; then
  echo "Rollback must reset the checkout before restarting containers" >&2
  exit 1
fi
marker_line=$(grep -m1 -n 'mv -f "$marker_tmp" "$marker"' "$SCRIPT" | cut -d: -f1 || true)
seed_line=$(grep -m1 -n 'docker compose exec -T auth-service /service seed-admin' "$SCRIPT" | cut -d: -f1 || true)
commit_line=$(grep -m1 -n 'transaction_committed=true' "$SCRIPT" | cut -d: -f1 || true)
if [[ -z "$marker_line" || -z "$seed_line" || -z "$commit_line" || "$marker_line" -ge "$seed_line" || "$seed_line" -ge "$commit_line" ]]; then
  echo "Marker and admin seed must remain rollback-covered until commit" >&2
  exit 1
fi
if grep -Fq 'REMOTE_SEED' "$SCRIPT"; then
  echo "Admin seed must be inside the rollback-covered transaction" >&2
  exit 1
fi
require_literal 'printf '\''%s\n'\'' "$PREVIOUS_SHA" >"$marker_tmp"'

if grep -Fq 'docker tag' "$SCRIPT"; then
  echo "Rollback must not retag immutable images as latest" >&2
  exit 1
fi
if grep -Fq 'docker compose pull' "$SCRIPT"; then
  echo "Deployment must pull explicit immutable image references" >&2
  exit 1
fi

bash "${SCRIPT_DIR}/deploy_remote_test.sh"
echo "deploy.sh: all tests passed"
