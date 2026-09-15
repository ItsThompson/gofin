#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
COMMON_SCRIPT="${SCRIPT_DIR}/deploy_remote_common.sh"
ROLLBACK_SCRIPT="${SCRIPT_DIR}/deploy_remote_rollback.sh"
TRANSACTION_SCRIPT="${SCRIPT_DIR}/deploy_remote_transaction.sh"
FIXTURE_COMMAND="${SCRIPT_DIR}/deploy_remote_fixture_command.sh"

remote_script=$(mktemp)
trap 'rm -f "$remote_script"' EXIT
cat "$COMMON_SCRIPT" "$ROLLBACK_SCRIPT" "$TRANSACTION_SCRIPT" >"$remote_script"
chmod +x "$remote_script"
for script in "$COMMON_SCRIPT" "$ROLLBACK_SCRIPT" "$TRANSACTION_SCRIPT"; do
  bash -n "$script"
done

PREVIOUS_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
DEPLOY_SHA=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb

run_failure_fixture() {
  local name=$1
  shift
  local expect_failure=true
  local expect_restore=true
  local expect_rollback=true
  local expect_removed=false
  local initial_head="$PREVIOUS_SHA"
  local root
  local fake_bin
  local events
  local marker
  local head
  local release
  local value
  local status
  local failure_flags=("${name}=1")
  local flag
  for flag in "$@"; do
    case "$flag" in
      success) expect_failure=false ;;
      no-restore) expect_restore=false ;;
      no-rollback) expect_rollback=false ;;
      removed-service) expect_removed=true; failure_flags+=("REMOVED_SERVICE=1") ;;
      head=*) initial_head=${flag#head=} ;;
      *) failure_flags+=("${flag}=1") ;;
    esac
  done
  root=$(mktemp -d)
  fake_bin=$(mktemp -d)
  events="$root/events"
  marker="$root/.deployed-sha"
  printf '%s\n' "$PREVIOUS_SHA" >"$marker"
  mkdir -p "$root/.git" "$root/deployments/cloudflare"
  printf '%s\n' "$initial_head" >"$root/.fake-head"
  printf 'SENTRY_DSN_BACKEND=backend\nSENTRY_DSN_FRONTEND=frontend\nSENTRY_RELEASE=%s\n' "$PREVIOUS_SHA" >"$root/.env"
  if [[ "$name" == RESERVED_ENV ]]; then
    printf 'DEPLOY_SHA=malicious\n' >>"$root/.env"
  fi
  printf 'old-app\n' >"$root/deployments/cloudflare/gofin-app.json"
  printf 'old-grafana\n' >"$root/deployments/cloudflare/gofin-grafana.json"
  printf 'old-cert\n' >"$root/deployments/cloudflare/cert.pem"
  printf 'app-config\n' >"$root/deployments/cloudflare/config-app.yml"
  printf 'grafana-config\n' >"$root/deployments/cloudflare/config-grafana.yml"
  : >"$events"
  cp "$FIXTURE_COMMAND" "$fake_bin/git"
  cp "$FIXTURE_COMMAND" "$fake_bin/docker"
  cp "$FIXTURE_COMMAND" "$fake_bin/envsubst"
  cp "$FIXTURE_COMMAND" "$fake_bin/mv"
  cp "$FIXTURE_COMMAND" "$fake_bin/cp"
  cp "$FIXTURE_COMMAND" "$fake_bin/sleep"
  chmod +x "$fake_bin"/*

  printf 'staged-app\n' >"$root/staged-app"
  printf 'staged-grafana\n' >"$root/staged-grafana"
  printf 'staged-cert\n' >"$root/staged-cert"
  cat >"$root/compose.json" <<'EOF'
{"services":{"expense-service":{"image":"ghcr.io/itsthompson/gofin/expense-service:latest"}}}
EOF
  if [[ "$expect_removed" == true ]]; then
    cat >"$root/compose-initial.json" <<'EOF'
{"services":{"expense-service":{"image":"ghcr.io/itsthompson/gofin/expense-service:latest"},"legacy-service":{"image":"ghcr.io/itsthompson/gofin/legacy-service:latest"}}}
EOF
  else
    cp "$root/compose.json" "$root/compose-initial.json"
  fi

  set +e
  env DEPLOY_ROOT="$root" \
  DEPLOY_SHA="$DEPLOY_SHA" \
  PREVIOUS_SHA="$PREVIOUS_SHA" \
  REPO_URL=https://example.invalid/gofin.git \
  CREDENTIAL_APP_TMP="$root/staged-app" \
  CREDENTIAL_GRAFANA_TMP="$root/staged-grafana" \
  CREDENTIAL_CERT_TMP="$root/staged-cert" \
  COMPOSE_CONFIG_FILE="$root/compose.json" \
  COMPOSE_INITIAL_CONFIG_FILE="$root/compose-initial.json" \
  FAKE_HEAD_FILE="$root/.fake-head" \
  FAKE_EVENTS_FILE="$events" \
  FAKE_MARKER_MOVED="$root/marker-move-failed" \
  REAL_MV="$(command -v mv)" \
  PATH="$fake_bin:$PATH" \
  "${failure_flags[@]}" \
  bash "$remote_script" >"$root/stdout" 2>"$root/stderr"
  status=$?
  set -e
  if [[ "$expect_failure" == true && "$status" -eq 0 ]]; then
    echo "Expected ${name} fixture to fail" >&2
    cat "$root/stdout" >&2
    cat "$root/stderr" >&2
    exit 1
  fi
  if [[ "$expect_failure" == false && "$status" -ne 0 ]]; then
    echo "Expected ${name} fixture to succeed" >&2
    cat "$root/stdout" >&2
    cat "$root/stderr" >&2
    exit 1
  fi

  head=$(<"$root/.fake-head")
  marker=$(<"$root/.deployed-sha")
  release=$(grep '^SENTRY_RELEASE=' "$root/.env")
  value=$(<"$root/deployments/cloudflare/gofin-app.json")
  if [[ "$expect_failure" == false ]]; then
    [[ "$head" == "$DEPLOY_SHA" ]] || { echo "${name}: target checkout was not retained" >&2; exit 1; }
    [[ "$marker" == "$DEPLOY_SHA" ]] || { echo "${name}: target marker was not recorded" >&2; exit 1; }
    [[ "$release" == "SENTRY_RELEASE=${DEPLOY_SHA}" ]] || { echo "${name}: target env was not retained" >&2; exit 1; }
    [[ "$value" == staged-app ]] || { echo "${name}: target credentials were not installed" >&2; exit 1; }
    if grep -q 'gofin-rollback' "$events"; then
      echo "${name}: unexpected rollback Compose start" >&2
      exit 1
    fi
  elif [[ "$expect_restore" == true ]]; then
    [[ "$head" == "$PREVIOUS_SHA" ]] || { echo "${name}: checkout was not restored" >&2; exit 1; }
    [[ "$marker" == "$PREVIOUS_SHA" ]] || { echo "${name}: marker was not restored" >&2; exit 1; }
    [[ "$release" == "SENTRY_RELEASE=${PREVIOUS_SHA}" ]] || { echo "${name}: env was not restored" >&2; exit 1; }
    [[ "$value" == old-app ]] || { echo "${name}: credentials were not restored" >&2; exit 1; }
  else
    grep -q 'FATAL: rollback failed; original deployment error was' "$root/stderr" || {
      echo "${name}: rollback failure was not fatal" >&2
      exit 1
    }
  fi
  if [[ "$expect_failure" == true && "$expect_rollback" == true && "$name" != FAIL_PULL && "$expect_restore" == true ]]; then
    grep -q 'gofin-rollback' "$events" || { echo "${name}: rollback Compose was not started" >&2; exit 1; }
    if [[ "$expect_removed" == true ]]; then
      grep -q 'legacy-service' "$events" || { echo "${name}: removed service was not included in rollback" >&2; exit 1; }
    fi
  fi
  rm -rf "$root" "$fake_bin"
}

run_failure_fixture RESERVED_ENV no-rollback
run_failure_fixture FAIL_PULL
run_failure_fixture FAIL_START
run_failure_fixture FAIL_START removed-service
run_failure_fixture FAIL_HEALTH
run_failure_fixture FAIL_MARKER
run_failure_fixture FAIL_SEED
run_failure_fixture FAIL_MARKER_RESTORE FAIL_SEED no-restore
run_failure_fixture FAIL_CREDENTIAL_RESTORE FAIL_CREDENTIAL_APP_RESTORE FAIL_SEED no-restore
run_failure_fixture FAIL_CREDENTIAL_RESTORE FAIL_CREDENTIAL_GRAFANA_RESTORE FAIL_SEED no-restore
run_failure_fixture FAIL_CREDENTIAL_RESTORE FAIL_CREDENTIAL_CERT_RESTORE FAIL_SEED no-restore
run_failure_fixture FAIL_SEED head=cccccccccccccccccccccccccccccccccccccccc no-rollback
run_failure_fixture MIGRATION success MIGRATE_LATEST

echo "deploy remote failure fixtures: all tests passed"
