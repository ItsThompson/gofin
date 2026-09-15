#!/usr/bin/env bash
# =============================================================================
# deploy.sh
#
# Non-interactive deployment script for gofin. Bootstraps a fresh VPS or
# updates an existing one. Intended to be run from a CI/CD pipeline
# (e.g., GitHub Actions on push to main) but can also be run manually.
#
# Prerequisites:
#   - SSH access to the server (key-based auth)
#   - Tunnel credentials in deployments/cloudflare/ (see docs/initial-setup.md)
#   - .env already configured on the server
#
# Usage:
#   ./scripts/deploy.sh <server-ip> [ssh-user]
#
# Example:
#   ./scripts/deploy.sh 65.108.42.100
#   ./scripts/deploy.sh 65.108.42.100 root
# =============================================================================
set -euo pipefail

SERVER_IP="${1:?Usage: $0 <server-ip> [ssh-user]}"
SSH_USER="${2:-root}"
SSH_TARGET="${SSH_USER}@${SERVER_IP}"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CREDENTIALS_DIR="${REPO_ROOT}/deployments/cloudflare"
REPO_URL="https://github.com/ItsThompson/gofin.git"
DEPLOY_SHA="${DEPLOY_SHA:?DEPLOY_SHA must be set to the commit SHA being deployed}"

if [[ ! "${DEPLOY_SHA}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "ERROR: DEPLOY_SHA must be exactly 40 lowercase hexadecimal characters." >&2
  exit 1
fi

# --- Preflight checks -------------------------------------------------------

echo "==> Preflight checks..."

if [[ ! -f "${CREDENTIALS_DIR}/gofin-app.json" ]]; then
  echo "ERROR: Tunnel credentials not found at ${CREDENTIALS_DIR}/gofin-app.json"
  echo "Run the tunnel setup runbook first: docs/initial-setup.md"
  exit 1
fi

if [[ ! -f "${CREDENTIALS_DIR}/gofin-grafana.json" ]]; then
  echo "ERROR: Tunnel credentials not found at ${CREDENTIALS_DIR}/gofin-grafana.json"
  echo "Run the tunnel setup runbook first: docs/initial-setup.md"
  exit 1
fi

if [[ ! -f "${CREDENTIALS_DIR}/cert.pem" ]]; then
  echo "ERROR: Origin certificate not found at ${CREDENTIALS_DIR}/cert.pem"
  echo "Run the tunnel setup runbook first: docs/initial-setup.md"
  exit 1
fi

echo "    Server:      ${SSH_TARGET}"
echo "    Credentials: ${CREDENTIALS_DIR}/"
echo ""

# --- Test SSH connection -----------------------------------------------------

echo "==> Testing SSH connection..."
if ! ssh -o ConnectTimeout=10 -o BatchMode=yes "${SSH_TARGET}" "echo ok" &>/dev/null; then
  echo "ERROR: Cannot SSH into ${SSH_TARGET}."
  echo "Ensure your SSH key is added to the server."
  exit 1
fi

# --- Install dependencies on server -----------------------------------------

echo "==> Installing Docker, Git, envsubst, and jq on the server..."
ssh "${SSH_TARGET}" bash <<'REMOTE_INSTALL'
set -euo pipefail

# Docker
if ! command -v docker &>/dev/null; then
  echo "  Installing Docker..."
  curl -fsSL https://get.docker.com | sh
else
  echo "  Docker already installed."
fi

# Git
if ! command -v git &>/dev/null; then
  echo "  Installing Git..."
  apt-get update -qq && apt-get install -y -qq git
else
  echo "  Git already installed."
fi

# envsubst (for rendering tunnel config templates)
if ! command -v envsubst &>/dev/null; then
  echo "  Installing envsubst (gettext-base)..."
  apt-get update -qq && apt-get install -y -qq gettext-base
else
  echo "  envsubst already installed."
fi

# jq (for parsing docker compose health status)
if ! command -v jq &>/dev/null; then
  echo "  Installing jq..."
  apt-get update -qq && apt-get install -y -qq jq
else
  echo "  jq already installed."
fi
REMOTE_INSTALL

# --- Configure Docker daemon (log rotation) ----------------------------------

echo "==> Configuring Docker daemon..."
scp "${REPO_ROOT}/deployments/docker/daemon.json" "${SSH_TARGET}:/tmp/daemon.json.new"
ssh "${SSH_TARGET}" bash <<'REMOTE_DAEMON'
set -euo pipefail
if ! cmp -s /tmp/daemon.json.new /etc/docker/daemon.json 2>/dev/null; then
  mv /tmp/daemon.json.new /etc/docker/daemon.json
  systemctl restart docker
  echo "  Docker daemon config updated and restarted."
else
  rm /tmp/daemon.json.new
  echo "  Docker daemon config unchanged, skipping restart."
fi
REMOTE_DAEMON

# --- Set up daily Docker prune (images + build cache) ------------------------

echo "==> Setting up daily Docker prune cron..."
ssh "${SSH_TARGET}" bash <<'REMOTE_CRON'
set -euo pipefail
cat > /etc/cron.daily/docker-prune <<'EOF'
#!/bin/sh
# Remove all unused images, build cache, and containerd content older than 72h
echo "$(date): starting docker prune" >> /var/log/docker-prune.log
docker system prune -af --filter "until=72h" >> /var/log/docker-prune.log 2>&1
EOF
chmod +x /etc/cron.daily/docker-prune
# Remove old weekly cron if it exists
rm -f /etc/cron.weekly/docker-prune
echo "  Daily prune cron installed."
REMOTE_CRON

# --- Capture the previous deployment marker before changing the checkout ------

PREVIOUS_SHA="$(ssh "${SSH_TARGET}" bash -s <<'REMOTE_PREVIOUS'
set -euo pipefail
marker=/opt/gofin/.deployed-sha
if [[ ! -f "$marker" ]]; then
  exit 0
fi
previous_sha="$(<"$marker")"
marker_bytes="$(wc -c <"$marker")"
if [[ ! "$previous_sha" =~ ^[0-9a-f]{40}$ || ( "$marker_bytes" -ne "${#previous_sha}" && "$marker_bytes" -ne $(( ${#previous_sha} + 1 )) ) ]]; then
  echo "ERROR: existing .deployed-sha must contain exactly one SHA line" >&2
  exit 1
fi
printf '%s\n' "$previous_sha"
REMOTE_PREVIOUS
)"
if [[ -n "$PREVIOUS_SHA" ]]; then
  echo "==> Previous deployment: ${PREVIOUS_SHA}"
else
  echo "==> No previous deployment marker. Rollback is unavailable for this first deploy."
fi

# --- Copy tunnel credentials to the server ----------------------------------

echo "==> Copying tunnel credentials and certificate to the server..."
CREDENTIAL_APP_TMP="/tmp/gofin-app-${DEPLOY_SHA}.json"
CREDENTIAL_GRAFANA_TMP="/tmp/gofin-grafana-${DEPLOY_SHA}.json"
CREDENTIAL_CERT_TMP="/tmp/gofin-cert-${DEPLOY_SHA}.pem"
cleanup_staged_credentials() {
  local status=$?
  ssh "${SSH_TARGET}" "rm -f -- '${CREDENTIAL_APP_TMP}' '${CREDENTIAL_GRAFANA_TMP}' '${CREDENTIAL_CERT_TMP}'" >/dev/null 2>&1 || true
  exit "$status"
}
trap cleanup_staged_credentials EXIT
scp "${CREDENTIALS_DIR}/gofin-app.json" "${SSH_TARGET}:${CREDENTIAL_APP_TMP}"
ssh "${SSH_TARGET}" "chmod 600 ${CREDENTIAL_APP_TMP}"
scp "${CREDENTIALS_DIR}/gofin-grafana.json" "${SSH_TARGET}:${CREDENTIAL_GRAFANA_TMP}"
ssh "${SSH_TARGET}" "chmod 600 ${CREDENTIAL_GRAFANA_TMP}"
scp "${CREDENTIALS_DIR}/cert.pem" "${SSH_TARGET}:${CREDENTIAL_CERT_TMP}"
ssh "${SSH_TARGET}" "chmod 600 ${CREDENTIAL_CERT_TMP}"

# --- Deploy the immutable image set ------------------------------------------

echo "==> Pulling immutable images and starting the stack..."
ssh "${SSH_TARGET}" env DEPLOY_SHA="${DEPLOY_SHA}" PREVIOUS_SHA="${PREVIOUS_SHA}" REPO_URL="${REPO_URL}" CREDENTIAL_APP_TMP="${CREDENTIAL_APP_TMP}" CREDENTIAL_GRAFANA_TMP="${CREDENTIAL_GRAFANA_TMP}" CREDENTIAL_CERT_TMP="${CREDENTIAL_CERT_TMP}" bash -s <<'REMOTE_DEPLOY'
set -euo pipefail

DEPLOY_ROOT="${DEPLOY_ROOT:-/opt/gofin}"
env_file="${DEPLOY_ROOT}/.env"
env_tmp=""
env_backup=""
target_override=""
rollback_override=""
rollback_refs=""
running_snapshot=""
marker_tmp=""
restore_tmp=""
credential_backup_dir=""
compose_config=""
expected_services=0
rollback_ref_count=0
resolved_rollback_ref=""
deployment_started=false
transaction_committed=false
rollback_in_progress=false
repo_created=false
checkout_restore_sha=""
marker="${DEPLOY_ROOT}/.deployed-sha"
credential_app="${DEPLOY_ROOT}/deployments/cloudflare/gofin-app.json"
credential_grafana="${DEPLOY_ROOT}/deployments/cloudflare/gofin-grafana.json"
credential_cert="${DEPLOY_ROOT}/deployments/cloudflare/cert.pem"

cleanup() {
  local temporary
  for temporary in "$env_tmp" "$env_backup" "$target_override" "$rollback_override" "$rollback_refs" "$running_snapshot" "$marker_tmp" "$restore_tmp" "$CREDENTIAL_APP_TMP" "$CREDENTIAL_GRAFANA_TMP" "$CREDENTIAL_CERT_TMP"; do
    [[ -z "$temporary" ]] || rm -f "$temporary"
  done
  [[ -z "$credential_backup_dir" ]] || rm -rf "$credential_backup_dir"
}

render_config() {
  local template=$1
  local output=$2
  local rendered_tmp
  if ! rendered_tmp="$(mktemp "${output}.tmp.XXXXXX")"; then
    echo "ERROR: temporary config file could not be created" >&2
    return 1
  fi
  if ! envsubst <"$template" >"$rendered_tmp"; then
    rm -f "$rendered_tmp" || true
    return 1
  fi
  if ! mv -f "$rendered_tmp" "$output"; then
    echo "ERROR: rendered config could not be installed" >&2
    rm -f "$rendered_tmp" || true
    return 1
  fi
}

resolve_rollback_ref() {
  local service=$1
  local current_ref=$2
  local current_id=$3
  local previous_ref
  local previous_id
  local current_ref_id

  if [[ ! "$current_ref" =~ :sha-[0-9a-f]{40}$ ]]; then
    if [[ -n "$PREVIOUS_SHA" && "$current_ref" =~ :latest$ ]]; then
      previous_ref="${current_ref%:latest}:sha-${PREVIOUS_SHA}"
      if ! docker pull "$previous_ref"; then
        echo "ERROR: previous image is unavailable for mutable service $service" >&2
        return 1
      fi
      if ! previous_id="$(docker image inspect --format '{{.Id}}' "$previous_ref")"; then
        echo "ERROR: previous image lookup failed for mutable service $service" >&2
        return 1
      fi
      if [[ "$previous_id" != "$current_id" ]]; then
        echo "ERROR: mutable image does not match the deployed marker for $service" >&2
        return 1
      fi
      current_ref="$previous_ref"
    else
      echo "ERROR: running service $service uses an unrecognized image tag: $current_ref" >&2
      return 1
    fi
  fi
  if ! current_ref_id="$(docker image inspect --format '{{.Id}}' "$current_ref")"; then
    echo "ERROR: rollback image lookup failed for $service" >&2
    return 1
  fi
  if [[ "$current_ref_id" != "$current_id" ]]; then
    echo "ERROR: immutable tag does not identify the running image for $service" >&2
    return 1
  fi
  resolved_rollback_ref="$current_ref"
}

wait_for_health() {
  local override_file=$1
  local attempt
  local rows
  local compose_args=(docker compose --profile tunnels -f docker-compose.yml -f "$override_file")
  for attempt in $(seq 1 12); do
    if ! rows="$("${compose_args[@]}" ps -a --format json)"; then
      echo "ERROR: Compose health query failed" >&2
      return 1
    fi
    if jq -e --argjson expected "$expected_services" '
      if type == "array" then . else [.] end
      | length == $expected
      and all(.[]; .State == "running" and ((.Health // "") == "" or .Health == "healthy"))
    ' <<<"$rows" >/dev/null; then
      return 0
    fi
    echo "  Waiting for healthy services (attempt ${attempt}/12)..."
    if ! sleep 5; then
      echo "ERROR: health-check delay failed" >&2
      return 1
    fi
  done
  return 1
}

restore_marker() {
  if [[ -n "$PREVIOUS_SHA" ]]; then
    if ! marker_tmp="$(mktemp "${marker}.rollback.XXXXXX")"; then
      echo "ERROR: rollback marker temporary file could not be created" >&2
      return 1
    fi
    if ! printf '%s\n' "$PREVIOUS_SHA" >"$marker_tmp"; then
      echo "ERROR: rollback marker could not be written" >&2
      return 1
    fi
    if ! mv -f "$marker_tmp" "$marker"; then
      echo "ERROR: rollback marker could not be restored" >&2
      return 1
    fi
    marker_tmp=""
  elif ! rm -f "$marker"; then
    echo "ERROR: rollback marker could not be removed" >&2
    return 1
  fi
}

rollback() {
  local service
  local old_ref
  local rollback_args
  local credential
  local destination
  local backup_name
  local service_list

  if [[ -n "$PREVIOUS_SHA" && -d "${DEPLOY_ROOT}/.git" ]]; then
    if ! git -C "$DEPLOY_ROOT" reset --hard "$PREVIOUS_SHA"; then
      echo "ERROR: rollback Git reset failed" >&2
      return 1
    fi
  elif [[ -n "$checkout_restore_sha" && -d "${DEPLOY_ROOT}/.git" ]]; then
    if ! git -C "$DEPLOY_ROOT" reset --hard "$checkout_restore_sha"; then
      echo "ERROR: rollback Git reset failed" >&2
      return 1
    fi
  elif [[ "$repo_created" == true ]]; then
    if ! rm -rf "$DEPLOY_ROOT"; then
      echo "ERROR: rollback checkout removal failed" >&2
      return 1
    fi
  fi

  if [[ -n "$env_backup" && -f "$env_backup" && -d "$DEPLOY_ROOT" ]]; then
    if ! restore_tmp="$(mktemp "${env_file}.rollback.XXXXXX")"; then
      echo "ERROR: rollback environment temporary file could not be created" >&2
      return 1
    fi
    if ! cp -p "$env_backup" "$restore_tmp" || ! mv -f "$restore_tmp" "$env_file"; then
      echo "ERROR: rollback environment restore failed" >&2
      return 1
    fi
    restore_tmp=""
  fi
  if [[ -n "$credential_backup_dir" && -d "$credential_backup_dir" && -d "$DEPLOY_ROOT" ]]; then
    for credential in app grafana cert; do
      destination="${credential_app}"
      backup_name="gofin-app.json"
      if [[ "$credential" == "grafana" ]]; then
        destination="$credential_grafana"
        backup_name="gofin-grafana.json"
      elif [[ "$credential" == "cert" ]]; then
        destination="$credential_cert"
        backup_name="cert.pem"
      fi
      if [[ -f "$credential_backup_dir/$backup_name" ]]; then
        if ! cp -p "$credential_backup_dir/$backup_name" "$destination"; then
          echo "ERROR: rollback credential restore failed: $destination" >&2
          return 1
        fi
      elif ! rm -f "$destination"; then
        echo "ERROR: rollback credential removal failed: $destination" >&2
        return 1
      fi
    done
  fi
  if ! restore_marker; then
    echo "ERROR: rollback marker restoration failed" >&2
    return 1
  fi

  if [[ ! -d "$DEPLOY_ROOT" ]]; then
    return 0
  fi

  if ! cd "$DEPLOY_ROOT"; then
    echo "ERROR: rollback checkout directory is unavailable" >&2
    return 1
  fi
  if [[ -f .env ]]; then
    set -a
    if ! source .env; then
      echo "ERROR: rollback environment could not be loaded" >&2
      return 1
    fi
    set +a
    if [[ -f deployments/cloudflare/config-app.yml ]]; then
      if ! render_config deployments/cloudflare/config-app.yml deployments/cloudflare/config-app.rendered.yml; then
        echo "ERROR: rollback app tunnel config render failed" >&2
        return 1
      fi
    fi
    if [[ -f deployments/cloudflare/config-grafana.yml ]]; then
      if ! render_config deployments/cloudflare/config-grafana.yml deployments/cloudflare/config-grafana.rendered.yml; then
        echo "ERROR: rollback Grafana tunnel config render failed" >&2
        return 1
      fi
    fi
  fi
  if [[ "$deployment_started" != true || "$rollback_ref_count" -eq 0 ]]; then
    return 0
  fi

  if [[ ! -f "$rollback_refs" ]]; then
    echo "ERROR: rollback image references are unavailable" >&2
    return 1
  fi
  if ! service_list="$(docker compose --profile tunnels config --services)"; then
    echo "ERROR: rollback Compose service list failed" >&2
    return 1
  fi
  if ! expected_services="$(printf '%s\n' "$service_list" | awk 'NF {count++} END {print count + 0}')"; then
    echo "ERROR: rollback service count failed" >&2
    return 1
  fi
  rollback_args=(docker compose --profile tunnels -f docker-compose.yml -f "$rollback_override" up -d --no-build --no-deps --remove-orphans)
  while IFS=$'\t' read -r service old_ref; do
    [[ -z "$service" ]] && continue
    if ! docker pull "$old_ref"; then
      echo "ERROR: rollback image is unavailable: $old_ref" >&2
      return 1
    fi
    rollback_args+=("$service")
  done <"$rollback_refs"
  if ! "${rollback_args[@]}"; then
    echo "ERROR: rollback Compose start failed" >&2
    return 1
  fi
  if ! wait_for_health "$rollback_override"; then
    echo "ERROR: rollback health check failed" >&2
    return 1
  fi
  echo "  Rollback completed with immutable image tags."
}

finish() {
  local status=$?
  local rollback_status=0
  if [[ "$status" -ne 0 && "$transaction_committed" != true && "$rollback_in_progress" != true ]]; then
    rollback_in_progress=true
    if rollback; then
      :
    else
      rollback_status=$?
      echo "FATAL: rollback failed; original deployment error was $status" >&2
    fi
  fi
  cleanup
  if [[ "$rollback_status" -ne 0 ]]; then
    exit "$rollback_status"
  fi
  exit "$status"
}
trap finish EXIT

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
REMOTE_DEPLOY

# --- Post-deploy cleanup (success path only) ---------------------------------

echo "==> Cleaning up stale Docker artifacts..."
if ! ssh "${SSH_TARGET}" bash -s <<'REMOTE_CLEANUP'
set -euo pipefail
if ! docker builder prune -af --filter "until=24h"; then
  echo "WARNING: builder prune failed (non-critical)" >&2
fi
if ! docker image prune -af --filter "until=72h"; then
  echo "WARNING: image prune failed (non-critical)" >&2
fi
journalctl --vacuum-size=50M 2>/dev/null || true
apt-get clean -y 2>/dev/null || true
: > /var/log/btmp 2>/dev/null || true
docker system df 2>/dev/null || true
REMOTE_CLEANUP
then
  echo "WARNING: cleanup failed after a successful deployment" >&2
fi

# --- Done --------------------------------------------------------------------

echo ""
echo "==========================================================================="
echo "  gofin deployed successfully!"
echo "==========================================================================="
echo ""
echo "  Deployment SHA: ${DEPLOY_SHA}"
echo ""
echo "  To redeploy after code changes:"
echo "    Push to main. CD handles build, push, and deploy automatically."
echo "==========================================================================="
