#!/usr/bin/env bash
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

validate_env_controls() {
  local line name
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    [[ -z "$line" || "$line" == \#* ]] && continue
    if [[ "$line" =~ ^export[[:space:]]+ ]]; then
      line="${line#export}"
      line="${line#"${line%%[![:space:]]*}"}"
    fi
    name="${line%%=*}"
    name="${name#"${name%%[![:space:]]*}"}"
    name="${name%"${name##*[![:space:]]}"}"
    if [[ ! "$name" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      echo "ERROR: .env contains an invalid variable name" >&2
      return 1
    fi
    case "$name" in
      DEPLOY_SHA|PREVIOUS_SHA|DEPLOY_ROOT|REPO_URL|CREDENTIAL_APP_TMP|CREDENTIAL_GRAFANA_TMP|CREDENTIAL_CERT_TMP|env_file|env_tmp|env_backup|target_override|rollback_override|rollback_refs|running_snapshot|marker_tmp|restore_tmp|credential_backup_dir|marker|credential_app|credential_grafana|credential_cert)
        echo "ERROR: .env contains reserved deployment variable: $name" >&2
        return 1
        ;;
    esac
  done <"$1"
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
