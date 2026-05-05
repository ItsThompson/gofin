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

echo "==> Installing Docker, Git, and envsubst on the server..."
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
REMOTE_INSTALL

# --- Clone or update the repo ------------------------------------------------

echo "==> Setting up repository on server..."
ssh "${SSH_TARGET}" bash <<REMOTE_REPO
set -euo pipefail
if [[ -d /opt/gofin/.git ]]; then
  echo "  Repo exists, pulling latest..."
  cd /opt/gofin && git pull
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
