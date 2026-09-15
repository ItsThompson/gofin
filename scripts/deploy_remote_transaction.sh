#!/usr/bin/env bash
set -euo pipefail

if ! declare -F rollback >/dev/null 2>&1; then
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  source "${SCRIPT_DIR}/deploy_remote_common.sh"
  source "${SCRIPT_DIR}/deploy_remote_rollback.sh"
fi

if [[ -n "$PREVIOUS_SHA" && ! "$PREVIOUS_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "ERROR: previous deployment SHA is invalid" >&2
  exit 1
fi

if [[ -d "${DEPLOY_ROOT}/.git" ]]; then
  echo "  Repo exists."
  if ! cd "$DEPLOY_ROOT"; then
    echo "ERROR: deployment checkout directory is unavailable" >&2
    exit 1
  fi
  if ! old_compose_config="$(docker compose --profile tunnels config --format json)"; then
    echo "ERROR: current Compose config could not be read" >&2
    exit 1
  fi
  if ! running_snapshot="$(mktemp /tmp/gofin-running-refs.XXXXXX)"; then
    echo "ERROR: running image snapshot could not be created" >&2
    exit 1
  fi
  running_services="$(jq -r '.services | to_entries[] | select(.value.image | startswith("ghcr.io/itsthompson/gofin/")) | .key' <<<"$old_compose_config")"
  while IFS= read -r service; do
    [[ -z "$service" ]] && continue
    container="$(docker compose --profile tunnels ps -q "$service")"
    [[ -z "$container" ]] && continue
    current_id="$(docker inspect --format '{{.Image}}' "$container")"
    current_ref="$(docker inspect --format '{{.Config.Image}}' "$container")"
    printf '%s\t%s\t%s\n' "$service" "$current_ref" "$current_id" >>"$running_snapshot"
  done <<<"$running_services"
elif [[ -e "$DEPLOY_ROOT" ]]; then
  echo "ERROR: $DEPLOY_ROOT exists but is not a Git checkout." >&2
  exit 1
else
  echo "  Cloning repo..."
  git clone --no-checkout "$REPO_URL" "$DEPLOY_ROOT"
  repo_created=true
fi
checkout_restore_sha="$(git -C "$DEPLOY_ROOT" rev-parse HEAD 2>/dev/null || true)"
if [[ -n "$PREVIOUS_SHA" ]]; then
  if ! git -C "$DEPLOY_ROOT" cat-file -e "${PREVIOUS_SHA}^{commit}"; then
    echo "ERROR: deployed marker does not name an available commit" >&2
    exit 1
  fi
  if [[ "$checkout_restore_sha" != "$PREVIOUS_SHA" ]]; then
    echo "ERROR: checkout does not match the deployed marker" >&2
    exit 1
  fi
fi
git -C "$DEPLOY_ROOT" fetch --no-tags origin main --force
if ! git -C "$DEPLOY_ROOT" cat-file -e "${DEPLOY_SHA}^{commit}" 2>/dev/null; then
  git -C "$DEPLOY_ROOT" fetch --no-tags origin "$DEPLOY_SHA"
fi
git -C "$DEPLOY_ROOT" cat-file -e "${DEPLOY_SHA}^{commit}"
git -C "$DEPLOY_ROOT" reset --hard "$DEPLOY_SHA"
mkdir -p "${DEPLOY_ROOT}/deployments/cloudflare"
credential_backup_dir="$(mktemp -d /tmp/gofin-credentials-backup.XXXXXX)"
for credential in app grafana cert; do
  source_path="$credential_app"
  backup_name="gofin-app.json"
  if [[ "$credential" == "grafana" ]]; then
    source_path="$credential_grafana"
    backup_name="gofin-grafana.json"
  elif [[ "$credential" == "cert" ]]; then
    source_path="$credential_cert"
    backup_name="cert.pem"
  fi
  if [[ -f "$source_path" ]]; then
    cp -p "$source_path" "$credential_backup_dir/$backup_name"
  fi
done
for staged_credential in "$CREDENTIAL_APP_TMP" "$CREDENTIAL_GRAFANA_TMP" "$CREDENTIAL_CERT_TMP"; do
  if [[ ! -f "$staged_credential" ]]; then
    echo "ERROR: staged tunnel credential is missing" >&2
    exit 1
  fi
done
mv -f "$CREDENTIAL_APP_TMP" "$credential_app"
mv -f "$CREDENTIAL_GRAFANA_TMP" "$credential_grafana"
mv -f "$CREDENTIAL_CERT_TMP" "$credential_cert"
cd "$DEPLOY_ROOT"

if [[ ! -f .env ]]; then
  echo "ERROR: $DEPLOY_ROOT/.env not found on server." >&2
  echo "Create it from $DEPLOY_ROOT/.env.example before deploying." >&2
  exit 1
fi
if ! validate_env_controls "$env_file"; then
  exit 1
fi

missing=""
for variable in SENTRY_DSN_BACKEND SENTRY_DSN_FRONTEND; do
  value="$(grep -m1 "^${variable}=" .env || true)"
  value="${value#*=}"
  value="${value%\"}"; value="${value#\"}"
  value="${value%\'}"; value="${value#\'}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  if [[ -z "$value" ]]; then
    missing="${missing} ${variable}"
  fi
done
if [[ -n "$missing" ]]; then
  echo "ERROR: missing or empty Sentry DSN in .env:${missing}" >&2
  exit 1
fi

compose_config="$(docker compose --profile tunnels config --format json)"
target_override="$(mktemp /tmp/gofin-target.XXXXXX.yml)"
rollback_override="$(mktemp /tmp/gofin-rollback.XXXXXX.yml)"
rollback_refs="$(mktemp /tmp/gofin-rollback-refs.XXXXXX)"
printf 'services:\n' >"$target_override"
printf 'services:\n' >"$rollback_override"
: >"$rollback_refs"

custom_count=0
while IFS=$'\t' read -r service image; do
  [[ -z "$service" ]] && continue
  if [[ ! "$service" =~ ^[A-Za-z0-9_.-]+$ || ! "$image" =~ ^ghcr\.io/itsthompson/gofin/[A-Za-z0-9_.-]+$ ]]; then
    echo "ERROR: unexpected custom image in Compose config" >&2
    exit 1
  fi
  [[ "$service" == "reporting" ]] && continue
  target_ref="${image}:sha-${DEPLOY_SHA}"
  printf '  %s:\n    image: %s\n' "$service" "$target_ref" >>"$target_override"
  if ! docker pull "$target_ref"; then
    echo "ERROR: immutable image is unavailable: $target_ref" >&2
    exit 1
  fi
  custom_count=$((custom_count + 1))
done < <(jq -r '.services | to_entries[] | select(.value.image | startswith("ghcr.io/itsthompson/gofin/")) | [.key, (.value.image | sub(":([^:]+)$"; ""))] | @tsv' <<<"$compose_config")

if [[ "$custom_count" -eq 0 ]]; then
  echo "ERROR: Compose config contains no custom images" >&2
  exit 1
fi

while IFS=$'\t' read -r service image; do
  [[ -z "$service" || "$service" == "reporting" ]] && continue
  target_ref="${image}:sha-${DEPLOY_SHA}"
  container="$(docker compose --profile tunnels ps -q "$service")"
  [[ -z "$container" ]] && continue
  current_id="$(docker inspect --format '{{.Image}}' "$container")"
  target_id="$(docker image inspect --format '{{.Id}}' "$target_ref")"
  current_ref="$(docker inspect --format '{{.Config.Image}}' "$container")"
  if ! resolve_rollback_ref "$service" "$current_ref" "$current_id"; then
    exit 1
  fi
  current_ref="$resolved_rollback_ref"
  printf '  %s:\n    image: %s\n' "$service" "$current_ref" >>"$rollback_override"
  printf '%s\t%s\n' "$service" "$current_ref" >>"$rollback_refs"
  rollback_ref_count=$((rollback_ref_count + 1))
  [[ "$current_id" == "$target_id" ]] && continue
done < <(jq -r '.services | to_entries[] | select(.value.image | startswith("ghcr.io/itsthompson/gofin/")) | [.key, (.value.image | sub(":([^:]+)$"; ""))] | @tsv' <<<"$compose_config")

if [[ -n "$running_snapshot" ]]; then
  while IFS=$'\t' read -r service current_ref current_id; do
    [[ -z "$service" ]] && continue
    if grep -Fq "$service"$'\t' "$rollback_refs"; then
      continue
    fi
    if [[ ! "$service" =~ ^[A-Za-z0-9_.-]+$ ]]; then
      echo "ERROR: unexpected service in running image snapshot" >&2
      exit 1
    fi
    if ! resolve_rollback_ref "$service" "$current_ref" "$current_id"; then
      exit 1
    fi
    current_ref="$resolved_rollback_ref"
    printf '  %s:\n    image: %s\n' "$service" "$current_ref" >>"$rollback_override"
    printf '%s\t%s\n' "$service" "$current_ref" >>"$rollback_refs"
    rollback_ref_count=$((rollback_ref_count + 1))
  done <"$running_snapshot"
fi

if ! jq -e '.services | has("expense-service")' <<<"$compose_config" >/dev/null; then
  echo "ERROR: expense-service is missing from the deployment Compose config" >&2
  exit 1
fi

# Pull every previous immutable image before changing host configuration.
while IFS=$'\t' read -r service old_ref; do
  [[ -z "$service" ]] && continue
  if ! docker pull "$old_ref"; then
    echo "ERROR: previous immutable image is unavailable: $old_ref" >&2
    exit 1
  fi
done <"$rollback_refs"

# Rewrite configuration only after every target image and rollback reference passes preflight.
env_file="$(readlink -f .env)"
env_backup="$(mktemp /tmp/gofin-env-backup.XXXXXX)"
cp -p "$env_file" "$env_backup"
env_tmp="$(mktemp "${env_file}.tmp.XXXXXX")"
old_lines="$(wc -l < "$env_file")"
old_release_lines="$(grep -c '^SENTRY_RELEASE=' "$env_file" || true)"
awk -v sha="$DEPLOY_SHA" '!/^SENTRY_RELEASE=/{print} END {print "SENTRY_RELEASE=" sha}' "$env_file" >"$env_tmp"
new_lines="$(wc -l < "$env_tmp")"
expected_lines=$((old_lines - old_release_lines + 1))
if [[ "$new_lines" -ne "$expected_lines" ]]; then
  echo "ERROR: refusing to replace .env: the rewrite lost lines" >&2
  exit 1
fi
mv -f "$env_tmp" "$env_file"
env_tmp=""

set -a
source .env
set +a
render_config deployments/cloudflare/config-app.yml deployments/cloudflare/config-app.rendered.yml
render_config deployments/cloudflare/config-grafana.yml deployments/cloudflare/config-grafana.rendered.yml
expected_services="$(docker compose --profile tunnels config --services | wc -l | tr -d ' ')"

deployment_started=true
if ! docker compose --profile tunnels -f docker-compose.yml -f "$target_override" up -d --no-build --remove-orphans; then
  echo "ERROR: immutable deployment failed to start" >&2
  exit 1
fi
if ! wait_for_health "$target_override"; then
  echo "ERROR: immutable deployment failed health checks" >&2
  exit 1
fi

marker_tmp="$(mktemp "${marker}.tmp.XXXXXX")"
printf '%s\n' "$DEPLOY_SHA" >"$marker_tmp"
mv -f "$marker_tmp" "$marker"
marker_tmp=""
echo "  Recorded deployed SHA: $DEPLOY_SHA"

for attempt in $(seq 1 30); do
  if docker compose exec -T auth-service /service seed-admin 2>/dev/null; then
    echo "  Admin user seeded."
    break
  fi
  if [[ "$attempt" -eq 30 ]]; then
    echo "ERROR: auth service did not become ready" >&2
    exit 1
  fi
  echo "  Waiting for auth service (attempt ${attempt}/30)..."
  sleep 2
done

transaction_committed=true
