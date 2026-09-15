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
if [[ ! "$previous_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "ERROR: existing .deployed-sha is not a 40-character lowercase SHA" >&2
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

# --- Clone or update the repo ------------------------------------------------

echo "==> Setting up repository on server at ${DEPLOY_SHA}..."
ssh "${SSH_TARGET}" env DEPLOY_SHA="${DEPLOY_SHA}" PREVIOUS_SHA="${PREVIOUS_SHA}" REPO_URL="${REPO_URL}" bash -s <<'REMOTE_REPO'
set -euo pipefail

if [[ -d /opt/gofin/.git ]]; then
  echo "  Repo exists."
elif [[ -e /opt/gofin ]]; then
  echo "ERROR: /opt/gofin exists but is not a Git checkout." >&2
  exit 1
else
  echo "  Cloning repo..."
  git clone --no-checkout "$REPO_URL" /opt/gofin
fi

git -C /opt/gofin fetch --no-tags origin main --force
if ! git -C /opt/gofin cat-file -e "${DEPLOY_SHA}^{commit}" 2>/dev/null; then
  git -C /opt/gofin fetch --no-tags origin "$DEPLOY_SHA"
fi
git -C /opt/gofin cat-file -e "${DEPLOY_SHA}^{commit}"
if [[ -n "$PREVIOUS_SHA" ]]; then
  git -C /opt/gofin cat-file -e "${PREVIOUS_SHA}^{commit}"
fi
git -C /opt/gofin reset --hard "$DEPLOY_SHA"
mkdir -p /opt/gofin/deployments/cloudflare
REMOTE_REPO

# --- Copy tunnel credentials to the server ----------------------------------

echo "==> Copying tunnel credentials and certificate to the server..."
scp "${CREDENTIALS_DIR}/gofin-app.json" "${SSH_TARGET}:/opt/gofin/deployments/cloudflare/gofin-app.json"
scp "${CREDENTIALS_DIR}/gofin-grafana.json" "${SSH_TARGET}:/opt/gofin/deployments/cloudflare/gofin-grafana.json"
scp "${CREDENTIALS_DIR}/cert.pem" "${SSH_TARGET}:/opt/gofin/deployments/cloudflare/cert.pem"
ssh "${SSH_TARGET}" "chmod 644 /opt/gofin/deployments/cloudflare/*.json /opt/gofin/deployments/cloudflare/cert.pem"

# --- Deploy the immutable image set ------------------------------------------

echo "==> Pulling immutable images and starting the stack..."
ssh "${SSH_TARGET}" env DEPLOY_SHA="${DEPLOY_SHA}" PREVIOUS_SHA="${PREVIOUS_SHA}" bash -s <<'REMOTE_DEPLOY'
set -euo pipefail
cd /opt/gofin

if [[ ! -f .env ]]; then
  echo "ERROR: /opt/gofin/.env not found on server." >&2
  echo "Create it from /opt/gofin/.env.example before deploying." >&2
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

env_tmp=""
env_backup=""
target_override=""
rollback_override=""
rollback_refs=""
marker_tmp=""
restore_tmp=""
trap 'rm -f "${env_tmp:-}" "${env_backup:-}" "${target_override:-}" "${rollback_override:-}" "${rollback_refs:-}" "${marker_tmp:-}" "${restore_tmp:-}"' EXIT

marker=/opt/gofin/.deployed-sha
if [[ -n "$PREVIOUS_SHA" && ! "$PREVIOUS_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "ERROR: previous deployment SHA is invalid" >&2
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

changed_count=0
while IFS=$'\t' read -r service image; do
  [[ -z "$service" || "$service" == "reporting" ]] && continue
  target_ref="${image}:sha-${DEPLOY_SHA}"
  container="$(docker compose --profile tunnels ps -q "$service")"
  [[ -z "$container" ]] && continue
  current_id="$(docker inspect --format '{{.Image}}' "$container")"
  target_id="$(docker image inspect --format '{{.Id}}' "$target_ref")"
  [[ "$current_id" == "$target_id" ]] && continue
  current_ref="$(docker inspect --format '{{.Config.Image}}' "$container")"
  if [[ ! "$current_ref" =~ :sha-[0-9a-f]{40}$ ]]; then
    echo "ERROR: changed running service $service uses a mutable image tag: $current_ref" >&2
    exit 1
  fi
  current_ref_id="$(docker image inspect --format '{{.Id}}' "$current_ref")"
  if [[ "$current_ref_id" != "$current_id" ]]; then
    echo "ERROR: immutable tag does not identify the running image for $service" >&2
    exit 1
  fi
  printf '  %s:\n    image: %s\n' "$service" "$current_ref" >>"$rollback_override"
  printf '%s\t%s\n' "$service" "$current_ref" >>"$rollback_refs"
  changed_count=$((changed_count + 1))
done < <(jq -r '.services | to_entries[] | select(.value.image | startswith("ghcr.io/itsthompson/gofin/")) | [.key, (.value.image | sub(":([^:]+)$"; ""))] | @tsv' <<<"$compose_config")

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
render_config() {
  local template=$1
  local output=$2
  local rendered_tmp
  rendered_tmp="$(mktemp "${output}.tmp.XXXXXX")"
  if ! envsubst <"$template" >"$rendered_tmp"; then
    rm -f "$rendered_tmp"
    return 1
  fi
  mv -f "$rendered_tmp" "$output"
}
render_config deployments/cloudflare/config-app.yml deployments/cloudflare/config-app.rendered.yml
render_config deployments/cloudflare/config-grafana.yml deployments/cloudflare/config-grafana.rendered.yml

expected_services="$(docker compose --profile tunnels config --services | wc -l | tr -d ' ')"
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
    sleep 5
  done
  return 1
}

rollback() {
  local service
  local old_ref
  if [[ "$changed_count" -eq 0 ]]; then
    echo "ERROR: no changed running service has an immutable rollback image" >&2
    return 1
  fi
  if [[ -z "$PREVIOUS_SHA" ]]; then
    echo "ERROR: cannot roll back without a previous deployment SHA" >&2
    return 1
  fi
  if ! git -C /opt/gofin reset --hard "$PREVIOUS_SHA"; then
    echo "ERROR: rollback Git reset failed" >&2
    return 1
  fi
  restore_tmp="$(mktemp "${env_file}.rollback.XXXXXX")"
  if ! cp -p "$env_backup" "$restore_tmp"; then
    echo "ERROR: rollback environment restore failed" >&2
    return 1
  fi
  if ! mv -f "$restore_tmp" "$env_file"; then
    echo "ERROR: rollback environment replace failed" >&2
    return 1
  fi
  restore_tmp=""
  set -a
  source .env
  set +a
  if ! render_config deployments/cloudflare/config-app.yml deployments/cloudflare/config-app.rendered.yml; then
    echo "ERROR: rollback app tunnel config render failed" >&2
    return 1
  fi
  if ! render_config deployments/cloudflare/config-grafana.yml deployments/cloudflare/config-grafana.rendered.yml; then
    echo "ERROR: rollback Grafana tunnel config render failed" >&2
    return 1
  fi
  expected_services="$(docker compose --profile tunnels config --services | wc -l | tr -d ' ')"
  local rollback_args
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

if ! docker compose --profile tunnels -f docker-compose.yml -f "$target_override" up -d --no-build --remove-orphans; then
  echo "ERROR: immutable deployment failed to start" >&2
  if ! rollback; then
    echo "ERROR: rollback failed after deployment start failure" >&2
  fi
  exit 1
fi
if ! wait_for_health "$target_override"; then
  echo "ERROR: immutable deployment failed health checks" >&2
  if ! rollback; then
    echo "ERROR: rollback failed after health-check failure" >&2
  fi
  exit 1
fi

marker_tmp="$(mktemp "${marker}.tmp.XXXXXX")"
printf '%s\n' "$DEPLOY_SHA" >"$marker_tmp"
mv -f "$marker_tmp" "$marker"
marker_tmp=""
echo "  Recorded deployed SHA: $DEPLOY_SHA"
REMOTE_DEPLOY

# --- Seed admin --------------------------------------------------------------

echo "==> Seeding the admin user..."
ssh "${SSH_TARGET}" bash -s <<'REMOTE_SEED'
set -euo pipefail
cd /opt/gofin
for attempt in $(seq 1 30); do
  if docker compose exec -T auth-service /service seed-admin 2>/dev/null; then
    echo "  Admin user seeded."
    exit 0
  fi
  echo "  Waiting for auth service (attempt ${attempt}/30)..."
  sleep 2
done
echo "ERROR: auth service did not become ready" >&2
exit 1
REMOTE_SEED

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
