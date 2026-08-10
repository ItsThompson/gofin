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

# --- Clone or update the repo ------------------------------------------------

echo "==> Setting up repository on server..."
ssh "${SSH_TARGET}" bash <<REMOTE_REPO
set -euo pipefail
if [[ -d /opt/gofin/.git ]]; then
  echo "  Repo exists, pulling latest..."
  cd /opt/gofin && git fetch origin && git reset --hard origin/main
else
  echo "  Cloning repo..."
  git clone ${REPO_URL} /opt/gofin
fi
mkdir -p /opt/gofin/deployments/cloudflare
REMOTE_REPO

# --- Copy tunnel credentials to server ---------------------------------------

echo "==> Copying tunnel credentials and certificate to server..."
scp "${CREDENTIALS_DIR}/gofin-app.json" "${SSH_TARGET}:/opt/gofin/deployments/cloudflare/gofin-app.json"
scp "${CREDENTIALS_DIR}/gofin-grafana.json" "${SSH_TARGET}:/opt/gofin/deployments/cloudflare/gofin-grafana.json"
scp "${CREDENTIALS_DIR}/cert.pem" "${SSH_TARGET}:/opt/gofin/deployments/cloudflare/cert.pem"
ssh "${SSH_TARGET}" "chmod 644 /opt/gofin/deployments/cloudflare/*.json /opt/gofin/deployments/cloudflare/cert.pem"

# --- Check .env exists on server ---------------------------------------------

echo "==> Checking .env on server..."
ssh "${SSH_TARGET}" bash <<'REMOTE_ENV'
set -euo pipefail
if [[ ! -f /opt/gofin/.env ]]; then
  echo "ERROR: /opt/gofin/.env not found on server."
  echo "SSH in and create it from .env.example:"
  echo "  cp /opt/gofin/.env.example /opt/gofin/.env && nano /opt/gofin/.env"
  exit 1
fi
echo "  .env found."

# Both Sentry DSNs must be present and non-empty. This deliberately fails closed
# while the application code fails open: .env.example ships both empty so CI and a
# fresh checkout send nothing, but a production deploy with an empty DSN reports
# nothing and looks exactly like a broken integration. There is no mechanism to
# push these from GitHub Secrets, so this check is the only interlock that does
# not depend on someone remembering. Do not "fix" the asymmetry.
#
# The value is unquoted and trimmed before the test: SENTRY_DSN_BACKEND="" is empty
# to both docker compose and source, and a hand-edited file is the one way either
# state arises.
MISSING=""
for VAR in SENTRY_DSN_BACKEND SENTRY_DSN_FRONTEND; do
  VALUE="$(grep -m1 "^${VAR}=" /opt/gofin/.env || true)"
  VALUE="${VALUE#*=}"
  VALUE="${VALUE%\"}"; VALUE="${VALUE#\"}"
  VALUE="${VALUE%\'}"; VALUE="${VALUE#\'}"
  VALUE="${VALUE#"${VALUE%%[![:space:]]*}"}"
  VALUE="${VALUE%"${VALUE##*[![:space:]]}"}"
  if [[ -z "${VALUE}" ]]; then
    MISSING="${MISSING} ${VAR}"
  fi
done

if [[ -n "${MISSING}" ]]; then
  echo "ERROR: missing or empty in /opt/gofin/.env:${MISSING}"
  echo "Every error would go unreported, which is indistinguishable from a broken"
  echo "integration. Add the values from the Sentry project's Client Keys page:"
  echo "  nano /opt/gofin/.env"
  exit 1
fi
echo "  Sentry DSNs present."
REMOTE_ENV

# --- Record the deploy SHA as the Sentry release -----------------------------

# Written before the REMOTE_START heredoc below, which is quoted and therefore
# does not interpolate, and before its `source .env`, so docker compose picks the
# value up. DEPLOY_SHA is interpolated by the local shell here, exactly as it is
# for the .deployed-sha write further down.
#
# The value stays a bare SHA: serverkit prefixes it as gofin-api@<sha> and the
# frontend's SSR entry as gofin-web@<sha>, so each prefix is applied exactly once.
if [[ -n "${DEPLOY_SHA:-}" ]]; then
  echo "==> Recording the deploy SHA as the Sentry release..."
  ssh "${SSH_TARGET}" bash <<REMOTE_RELEASE
set -euo pipefail
# Resolved, so replacing the file cannot turn a symlinked .env into a regular file.
ENV_FILE="\$(readlink -f /opt/gofin/.env)"
TMP_FILE="\${ENV_FILE}.deploy-tmp"

# Rewritten rather than appended, so the file cannot grow a line per deploy, and
# through a temp copy so a partial write cannot truncate the live file. The copy is
# what carries the original mode across the rename, and it also repairs a file
# saved without a trailing newline, because grep always terminates its output.
cp -p "\${ENV_FILE}" "\${TMP_FILE}"
{ grep -v '^SENTRY_RELEASE=' "\${ENV_FILE}" || true; } > "\${TMP_FILE}"
printf 'SENTRY_RELEASE=%s\n' '${DEPLOY_SHA}' >> "\${TMP_FILE}"

# grep exits 1 on zero matches, which is the common case here, so its status says
# nothing. Compare line counts instead: the rewrite must keep every line that is
# not the release, plus the one it writes.
KEPT=\$(grep -vc '^SENTRY_RELEASE=' "\${ENV_FILE}" || true)
if [[ "\$(wc -l < "\${TMP_FILE}")" -ne "\$((KEPT + 1))" ]]; then
  echo "ERROR: refusing to replace \${ENV_FILE}: the rewrite lost lines"
  rm -f "\${TMP_FILE}"
  exit 1
fi

mv "\${TMP_FILE}" "\${ENV_FILE}"
grep -n '^SENTRY_RELEASE=' "\${ENV_FILE}"
REMOTE_RELEASE
fi

# --- Pull images and start ---------------------------------------------------

echo "==> Rendering tunnel configs, pulling images, and starting the stack..."
ssh "${SSH_TARGET}" bash <<'REMOTE_START'
set -euo pipefail
cd /opt/gofin

# Render tunnel config templates from .env
set -a
source .env
set +a
envsubst < deployments/cloudflare/config-app.yml > deployments/cloudflare/config-app.rendered.yml
envsubst < deployments/cloudflare/config-grafana.yml > deployments/cloudflare/config-grafana.rendered.yml

# Pull pre-built images from GHCR
docker compose pull

# Start/recreate containers with pulled images
docker compose --profile tunnels up -d
REMOTE_START

# --- Post-deploy health check ------------------------------------------------

echo "==> Running post-deploy health checks..."
HEALTH_OK=false
for i in $(seq 1 12); do
  UNHEALTHY=$(ssh "${SSH_TARGET}" bash <<'REMOTE_HEALTH'
set -euo pipefail
cd /opt/gofin
docker compose ps --format json | jq -r 'select(.Health != "healthy" and .Health != "") | .Service'
REMOTE_HEALTH
  )
  if [ -z "${UNHEALTHY}" ]; then
    HEALTH_OK=true
    break
  fi
  echo "  Waiting for: ${UNHEALTHY//$'\n'/, } (attempt ${i}/12)"
  sleep 5
done

if [ "${HEALTH_OK}" = "true" ]; then
  echo "  All services healthy."
  # Record deployed SHA for future rollback
  if [ -n "${DEPLOY_SHA:-}" ]; then
    ssh "${SSH_TARGET}" "echo '${DEPLOY_SHA}' > /opt/gofin/.deployed-sha"
    echo "  Recorded deployed SHA: ${DEPLOY_SHA}"
  fi
else
  echo "ERROR: Health checks failed after 60 seconds."
  # Attempt rollback if previous SHA is recorded
  PREV_SHA=$(ssh "${SSH_TARGET}" "cat /opt/gofin/.deployed-sha 2>/dev/null || true")
  if [ -n "${PREV_SHA}" ]; then
    echo "==> Rolling back to previous SHA: ${PREV_SHA}"
    ssh "${SSH_TARGET}" bash <<REMOTE_ROLLBACK
set -euo pipefail
cd /opt/gofin

# Pull previous images by SHA tag
for svc in auth-service finance-service datarights-service api-gateway mfe; do
  docker pull "ghcr.io/itsthompson/gofin/\${svc}:sha-${PREV_SHA}" || true
  docker tag "ghcr.io/itsthompson/gofin/\${svc}:sha-${PREV_SHA}" "ghcr.io/itsthompson/gofin/\${svc}:latest" || true
done

docker compose --profile tunnels up -d
REMOTE_ROLLBACK
    echo "  Rollback complete. Previous version restored."
  else
    echo "  No previous SHA recorded: cannot rollback automatically."
  fi
  exit 1
fi

# --- Seed admin --------------------------------------------------------------

echo "==> Waiting for services to be healthy..."
ssh "${SSH_TARGET}" bash <<'REMOTE_SEED'
set -euo pipefail
cd /opt/gofin

# Wait for auth service to be ready (up to 60 seconds)
for i in $(seq 1 30); do
  if docker compose exec -T auth-service /service seed-admin 2>/dev/null; then
    echo "  Admin user seeded."
    exit 0
  fi
  echo "  Waiting for auth service... (attempt ${i}/30)"
  sleep 2
done

echo "ERROR: Auth service did not become healthy in time."
exit 1
REMOTE_SEED

# --- Post-deploy cleanup (success path only) ---------------------------------

echo "==> Cleaning up stale Docker artifacts..."
ssh "${SSH_TARGET}" bash <<'REMOTE_CLEANUP' || true

# Remove build cache older than 24 hours
echo "  Pruning build cache..."
docker builder prune -af --filter "until=24h" 2>&1 | tail -1 || echo "  WARNING: builder prune failed (non-critical)"

# Remove unused images older than 72 hours (keeps images used by running containers)
echo "  Pruning unused images..."
docker image prune -af --filter "until=72h" 2>&1 | tail -1 || echo "  WARNING: image prune failed (non-critical)"

# System-level cleanup
echo "  Running system cleanup..."
journalctl --vacuum-size=50M 2>/dev/null || true
apt-get clean -y 2>/dev/null || true
: > /var/log/btmp 2>/dev/null || true

# Report final state
echo "  Docker disk usage after cleanup:"
docker system df 2>/dev/null || true
REMOTE_CLEANUP

# --- Done --------------------------------------------------------------------

DOMAIN=$(ssh "${SSH_TARGET}" "grep CF_APP_HOSTNAME /opt/gofin/.env | cut -d= -f2" 2>/dev/null || echo "your-domain")
GRAFANA_DOMAIN=$(ssh "${SSH_TARGET}" "grep CF_GRAFANA_HOSTNAME /opt/gofin/.env | cut -d= -f2" 2>/dev/null || echo "grafana.your-domain")

echo ""
echo "==========================================================================="
echo "  gofin deployed successfully!"
echo "==========================================================================="
echo ""
echo "  App:     https://${DOMAIN:-your-domain}"
echo "  Grafana: https://${GRAFANA_DOMAIN:-grafana.your-domain}"
echo ""
echo "  To redeploy after code changes:"
echo "    Push to main — CD workflow handles build, push, and deploy automatically."
echo "==========================================================================="
