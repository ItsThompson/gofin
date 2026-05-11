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
#   - Tunnel credentials in deployments/cloudflare/ (see docs/tunnel-setup.md)
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
  echo "Run the tunnel setup runbook first: docs/tunnel-setup.md"
  exit 1
fi

if [[ ! -f "${CREDENTIALS_DIR}/gofin-grafana.json" ]]; then
  echo "ERROR: Tunnel credentials not found at ${CREDENTIALS_DIR}/gofin-grafana.json"
  echo "Run the tunnel setup runbook first: docs/tunnel-setup.md"
  exit 1
fi

if [[ ! -f "${CREDENTIALS_DIR}/cert.pem" ]]; then
  echo "ERROR: Origin certificate not found at ${CREDENTIALS_DIR}/cert.pem"
  echo "Run the tunnel setup runbook first: docs/tunnel-setup.md"
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

# --- Set up weekly Docker image prune ----------------------------------------

echo "==> Setting up weekly image prune cron..."
ssh "${SSH_TARGET}" bash <<'REMOTE_CRON'
set -euo pipefail
cat > /etc/cron.weekly/docker-prune <<'EOF'
#!/bin/sh
# Remove unused images older than 72 hours (keeps images used by running containers)
docker image prune -af --filter "until=72h" >> /var/log/docker-prune.log 2>&1
EOF
chmod +x /etc/cron.weekly/docker-prune
echo "  Weekly prune cron installed."
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
REMOTE_ENV

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
