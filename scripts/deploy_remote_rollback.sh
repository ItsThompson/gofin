#!/usr/bin/env bash
set -euo pipefail

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
    if ! validate_env_controls "$env_file"; then
      echo "ERROR: rollback environment contains reserved deployment variables" >&2
      return 1
    fi
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
