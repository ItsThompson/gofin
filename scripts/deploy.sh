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

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}/.."
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
ssh "${SSH_TARGET}" bash -s < "${SCRIPT_DIR}/deploy_remote_install.sh"

# --- Configure Docker daemon (log rotation) ----------------------------------
echo "==> Configuring Docker daemon..."
scp "${REPO_ROOT}/deployments/docker/daemon.json" "${SSH_TARGET}:/tmp/daemon.json.new"
ssh "${SSH_TARGET}" bash -s < "${SCRIPT_DIR}/deploy_remote_daemon.sh"

# --- Set up daily Docker prune (images + build cache) ------------------------

echo "==> Setting up daily Docker prune cron..."
ssh "${SSH_TARGET}" bash -s < "${SCRIPT_DIR}/deploy_remote_cron.sh"

# --- Capture the previous deployment marker before changing the checkout ------

PREVIOUS_SHA="$(ssh "${SSH_TARGET}" bash -s < "${SCRIPT_DIR}/deploy_remote_previous.sh")"
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
cat \
  "${SCRIPT_DIR}/deploy_remote_common.sh" \
  "${SCRIPT_DIR}/deploy_remote_rollback.sh" \
  "${SCRIPT_DIR}/deploy_remote_transaction.sh" \
  | ssh "${SSH_TARGET}" env DEPLOY_SHA="${DEPLOY_SHA}" PREVIOUS_SHA="${PREVIOUS_SHA}" REPO_URL="${REPO_URL}" CREDENTIAL_APP_TMP="${CREDENTIAL_APP_TMP}" CREDENTIAL_GRAFANA_TMP="${CREDENTIAL_GRAFANA_TMP}" CREDENTIAL_CERT_TMP="${CREDENTIAL_CERT_TMP}" bash -s

# --- Post-deploy cleanup (success path only) ---------------------------------

echo "==> Cleaning up stale Docker artifacts..."
if ! ssh "${SSH_TARGET}" bash -s < "${SCRIPT_DIR}/deploy_remote_cleanup.sh"; then
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
