#!/usr/bin/env bash
# grafana-cloud-migrate.sh
#
# Prepares self-hosted Grafana dashboards for import into Grafana Cloud.
# Transforms datasource UIDs from the hardcoded "gofin-prometheus" to a
# templated variable that auto-resolves in Grafana Cloud.
#
# Usage:
#   ./scripts/grafana-cloud-migrate.sh
#
# Output:
#   monitoring/grafana-cloud/dashboards/*.json  (import-ready dashboards)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

SOURCE_DIR="${PROJECT_ROOT}/monitoring/grafana/dashboards"
OUTPUT_DIR="${PROJECT_ROOT}/monitoring/grafana-cloud/dashboards"

mkdir -p "${OUTPUT_DIR}"

echo "=== Grafana Cloud Dashboard Migration ==="
echo ""
echo "Source: ${SOURCE_DIR}"
echo "Output: ${OUTPUT_DIR}"
echo ""

if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required. Install with: brew install jq"
  exit 1
fi

count=0
for dashboard_file in "${SOURCE_DIR}"/*.json; do
  [ -f "${dashboard_file}" ] || continue

  filename="$(basename "${dashboard_file}")"
  title="$(jq -r '.title // "unknown"' "${dashboard_file}")"

  # Transform for Grafana Cloud:
  # 1. Replace hardcoded datasource UID with variable reference
  # 2. Add __inputs declaration for datasource variable binding
  # 3. Remove id (Cloud assigns its own)
  # 4. Remove uid (avoid conflicts, Cloud will assign)
  jq '
    # Add __inputs for datasource variable binding on import
    . + {
      "__inputs": [
        {
          "name": "DS_PROMETHEUS",
          "label": "Prometheus",
          "description": "Grafana Cloud Prometheus (Mimir) datasource",
          "type": "datasource",
          "pluginId": "prometheus",
          "pluginName": "Prometheus"
        }
      ]
    }
    # Replace all hardcoded datasource UIDs with the input variable
    | walk(
      if type == "object" and .uid? == "gofin-prometheus" and .type? == "prometheus"
      then . + {"uid": "${DS_PROMETHEUS}"}
      else .
      end
    )
    # Remove fields that conflict with Cloud import
    | del(.id)
    | .uid = null
  ' "${dashboard_file}" > "${OUTPUT_DIR}/${filename}"

  echo "  ✓ ${filename} (${title})"
  count=$((count + 1))
done

echo ""
echo "Migrated ${count} dashboards to ${OUTPUT_DIR}"
echo ""
echo "Next steps:"
echo "  1. Log in to Grafana Cloud"
echo "  2. Go to Dashboards → Import"
echo "  3. Upload each JSON file from ${OUTPUT_DIR}"
echo "  4. Select your Grafana Cloud Prometheus datasource when prompted"
echo ""
echo "See scripts/grafana-cloud-provision.sh for the full provisioning guide."
