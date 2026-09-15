#!/usr/bin/env bash
set -euo pipefail

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: docker is required" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required" >&2
  exit 1
fi

if ! compose_config=$(docker compose --profile reporting config --format json 2>/dev/null); then
  echo "ERROR: docker compose could not render the reporting profile" >&2
  exit 1
fi

redacted_config() {
  jq '
    walk(
      if type == "object" and has("environment") then
        .environment = (
          if (.environment | type) == "object" then
            .environment | with_entries(.value = "<redacted>")
          elif (.environment | type) == "array" then
            .environment | map(sub("=.*$"; "=<redacted>"))
          else .environment
          end
        )
      else .
      end
    )
  '
}

if ! jq -e '
  .services.reporting as $reporting
  | ($reporting != null)
  and (($reporting.image | split(":")[0]) == "ghcr.io/itsthompson/gofin/reporting")
  and (($reporting.profiles // []) | index("reporting") != null)
  and (($reporting.networks // {}) | keys == ["compute-net"])
  and ($reporting.ports == null)
  and ($reporting.restart == null)
  and ($reporting.depends_on == null)
  and ($reporting.build.context | endswith("/services"))
  and ($reporting.build.dockerfile == "reporting/Dockerfile")
  and ($reporting.environment.AUTH_SERVICE_ADDR == "auth-service:9081")
  and ($reporting.environment.EXPENSE_SERVICE_ADDR == "expense-service:9082")
  and ($reporting.environment.FINANCE_SERVICE_ADDR == "finance-service:9083")
  and ($reporting.environment.DATARIGHTS_SERVICE_ADDR == "datarights-service:9084")
  and ($reporting.deploy.resources.limits.cpus != null)
  and ($reporting.deploy.resources.limits.memory != null)
  and ((($reporting.environment // {}) | keys | any(test("(?i)(db|database|postgres|immudb|password|passwd|credential)")) | not))
' <<<"$compose_config" >/dev/null; then
  echo "ERROR: reporting service does not satisfy its isolated Compose contract" >&2
  redacted_config <<<"$compose_config" >&2
  exit 1
fi

# Compose resolves interpolated environment values before emitting JSON. Never
# print those values, including values belonging to unrelated services.
redacted_config <<<"$compose_config"
