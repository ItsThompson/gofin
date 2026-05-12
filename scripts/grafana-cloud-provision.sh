#!/usr/bin/env bash
# grafana-cloud-provision.sh
#
# Interactive provisioning guide for Grafana Cloud migration.
# Walks through each step, pausing for manual actions.
#
# Prerequisites:
#   - jq installed (brew install jq)
#   - Access to the VPS via SSH
#   - Access to monitoring/grafana-cloud/ directory (run grafana-cloud-migrate.sh first)
#
# Usage:
#   ./scripts/grafana-cloud-provision.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CLOUD_DIR="${PROJECT_ROOT}/monitoring/grafana-cloud"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

step=0

print_step() {
  step=$((step + 1))
  echo ""
  echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${GREEN}Step ${step}: ${1}${NC}"
  echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
}

pause() {
  echo ""
  echo -e "${YELLOW}Press Enter when done...${NC}"
  read -r
}

# ─────────────────────────────────────────────────────────────────────────────

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║          Grafana Cloud Provisioning Guide                    ║"
echo "║                                                              ║"
echo "║  This script walks through the manual steps needed to        ║"
echo "║  provision Grafana Cloud and migrate from self-hosted.       ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# ─────────────────────────────────────────────────────────────────────────────

print_step "Create Grafana Cloud Account"

cat << 'EOF'
  1. Go to https://grafana.com/products/cloud/
  2. Click "Create free account"
  3. Sign up (GitHub OAuth recommended)
  4. Choose a stack name (e.g., "gofin")
  5. Select a region close to your VPS (e.g., EU if Hetzner Germany)

  Free tier includes:
    - 10,000 active metric series (GoFin uses ~500-1000)
    - 50 GB logs/month (GoFin uses ~1 GB)
    - 3 users (GoFin needs 1)
    - 14-day retention (upgrade from current 3-day local)
    - 100 alert rules (GoFin uses ~10)
EOF

pause

# ─────────────────────────────────────────────────────────────────────────────

print_step "Get Remote-Write Credentials"

cat << 'EOF'
  1. In Grafana Cloud, go to: Connections → Data sources
  2. Find "grafanacloud-<stack>-prom" (the hosted Prometheus/Mimir instance)
  3. Click on it → scroll to "Remote Write Config" or "Using Grafana Alloy"
  4. Note down:
     - Remote write endpoint URL (e.g., https://prometheus-prod-XX-XX.grafana.net/api/prom/push)
     - Username (numeric ID)
  5. Generate an API key:
     - Go to grafana.com → My Account → Security → API Keys
     - Create key with "MetricsPublisher" role
     - Copy the key (starts with "glc_")
EOF

echo ""
echo -e "${YELLOW}Enter the remote-write URL:${NC}"
read -r REMOTE_WRITE_URL
echo -e "${YELLOW}Enter the remote-write username:${NC}"
read -r REMOTE_WRITE_USER
echo -e "${YELLOW}Enter the API key (glc_...):${NC}"
read -rs REMOTE_WRITE_KEY
echo ""

echo ""
echo "Credentials collected. Add these to VPS .env:"
echo ""
echo "  GRAFANA_REMOTE_WRITE_URL=${REMOTE_WRITE_URL}"
echo "  GRAFANA_REMOTE_WRITE_USER=${REMOTE_WRITE_USER}"
echo "  GRAFANA_REMOTE_WRITE_KEY=${REMOTE_WRITE_KEY}"
echo ""
echo "Run on VPS:"
echo "  cat >> ~/.env << EOL"
echo "  GRAFANA_REMOTE_WRITE_URL=${REMOTE_WRITE_URL}"
echo "  GRAFANA_REMOTE_WRITE_USER=${REMOTE_WRITE_USER}"
echo "  GRAFANA_REMOTE_WRITE_KEY=${REMOTE_WRITE_KEY}"
echo "  EOL"

pause

# ─────────────────────────────────────────────────────────────────────────────

print_step "Import Dashboards"

cat << EOF
  Dashboard JSON files are ready in: ${CLOUD_DIR}/dashboards/

  For each dashboard:
    1. Go to Grafana Cloud → Dashboards → New → Import
    2. Click "Upload JSON file"
    3. Select the dashboard JSON
    4. When prompted for datasource, select your Grafana Cloud Prometheus
    5. Click "Import"

  Dashboards to import:
EOF

for f in "${CLOUD_DIR}/dashboards/"*.json; do
  title=$(jq -r '.title' "$f")
  echo "    - $(basename "$f") → ${title}"
done

echo ""
echo "  After import, verify each dashboard loads without errors."

pause

# ─────────────────────────────────────────────────────────────────────────────

print_step "Configure Discord Contact Point"

cat << 'EOF'
  1. Go to Alerting → Contact points
  2. Click "New contact point"
  3. Name: "discord-critical"
  4. Integration type: Discord
  5. Webhook URL: (paste your Discord webhook URL)
  6. Title: 🚨 [CRITICAL] {{ .GroupLabels.alertname }}
  7. Message: {{ range .Alerts }}**{{ .Annotations.summary }}**\n{{ end }}
  8. Check "Send resolved messages"
  9. Save & test

  Repeat for "discord-warning":
  1. Name: "discord-warning"
  2. Title: ⚠️ [WARNING] {{ .GroupLabels.alertname }}
  3. Message: {{ range .Alerts }}{{ .Annotations.summary }}\n{{ end }}
  4. Check "Send resolved messages"
EOF

pause

# ─────────────────────────────────────────────────────────────────────────────

print_step "Configure Notification Policy"

cat << 'EOF'
  1. Go to Alerting → Notification policies
  2. Edit the default policy:
     - Default contact point: "discord-warning"
  3. Add a nested policy:
     - Matching labels: severity = critical
     - Contact point: "discord-critical"
     - Group wait: 30s
     - Group interval: 5m
     - Repeat interval: 30m
  4. Save
EOF

pause

# ─────────────────────────────────────────────────────────────────────────────

print_step "Import Alert Rules"

cat << EOF
  Alert rules file: ${CLOUD_DIR}/alerts/alert-rules.yml

  Option A (UI import):
    1. Go to Alerting → Alert rules
    2. Create a folder called "GoFin"
    3. Manually create each rule using the PromQL expressions from the file
       (The UI currently doesn't support bulk YAML import)

  Option B (API import, if Grafana Cloud API is available):
    curl -X POST \\
      -H "Authorization: Bearer \${GRAFANA_CLOUD_API_KEY}" \\
      -H "Content-Type: application/yaml" \\
      -d @${CLOUD_DIR}/alerts/alert-rules.yml \\
      https://\${GRAFANA_CLOUD_STACK}.grafana.net/api/v1/provisioning/alert-rules

  Critical rules to verify:
    - HostDiskFillingUp (this alert triggered the entire optimization epic)
    - HostDiskAlmostFull
    - ServiceDown
    - HighErrorRate
EOF

pause

# ─────────────────────────────────────────────────────────────────────────────

print_step "Import Recording Rules"

cat << EOF
  Recording rules file: ${CLOUD_DIR}/alerts/recording-rules.yml

  Option A (Mimir ruler API):
    curl -X POST \\
      -H "Authorization: Bearer \${GRAFANA_CLOUD_API_KEY}" \\
      -H "Content-Type: application/yaml" \\
      --data-binary @${CLOUD_DIR}/alerts/recording-rules.yml \\
      https://\${GRAFANA_CLOUD_STACK}.grafana.net/api/v1/rules/gofin

  Option B (UI):
    1. Go to Alerting → Recording rules (or use the Mimir config UI)
    2. Create each recording rule group manually

  Recording rule groups:
    - http-latency (p50, p95, p99 by service)
    - http-rates (request rate, error rate by service)
    - db-latency (p95 query duration)
    - grpc-latency (p95 gRPC duration)
    - service-specific (finance path latency, expense query latency)
EOF

pause

# ─────────────────────────────────────────────────────────────────────────────

print_step "Verify HostDiskFillingUp Alert"

cat << 'EOF'
  This is the most critical alert: it triggered the entire Docker optimization epic.

  Verification:
    1. Go to Alerting → Alert rules → GoFin → hardware
    2. Find "HostDiskFillingUp"
    3. Check the rule evaluates correctly (status should be "Normal" or "Pending")
    4. The expression uses predict_linear() over a 6h window
    5. If metrics are flowing from Alloy, the rule should evaluate without errors

  To force-test the Discord integration:
    1. Create a temporary alert rule with: vector(1) > 0 (always firing)
    2. Wait for the alert to fire → verify Discord message arrives
    3. Delete the temporary rule

  If HostDiskFillingUp shows "No Data":
    - Metrics may not be flowing yet (Alloy not deployed: see ticket #11)
    - This is expected until Alloy is running on the VPS
EOF

pause

# ─────────────────────────────────────────────────────────────────────────────

print_step "Verification Checklist"

echo "  Verify each item:"
echo ""
echo "  [ ] Grafana Cloud account active (check: can log in)"
echo "  [ ] Remote-write URL obtained (check: URL in .env)"
echo "  [ ] Remote-write credentials work (check: user + key in .env)"
echo "  [ ] All 7 dashboards imported (check: visible in Dashboards list)"
echo "  [ ] Dashboards load without datasource errors"
echo "  [ ] Discord contact point configured (check: test notification sent)"
echo "  [ ] Notification policy routes critical → discord-critical"
echo "  [ ] All 12 alert rules imported (check: Alerting → Alert rules)"
echo "  [ ] HostDiskFillingUp rule evaluates without errors"
echo "  [ ] Recording rules imported (check: Mimir rules endpoint)"
echo ""
echo -e "${GREEN}Done! Grafana Cloud is provisioned.${NC}"
echo ""
echo "Next: Deploy Grafana Alloy (ticket #11) to start sending metrics."
