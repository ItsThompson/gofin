#!/usr/bin/env bash
set -euo pipefail

command_name=$(basename "$0")
case "$command_name" in
  git)
    repo_root=""
    if [[ "${1:-}" == "-C" ]]; then
      repo_root=$2
      shift 2
    fi
    case "${1:-}" in
      rev-parse)
        cat "${FAKE_HEAD_FILE}"
        ;;
      fetch|cat-file)
        ;;
      reset)
        printf '%s\n' "${*: -1}" >"$FAKE_HEAD_FILE"
        ;;
      *)
        echo "unexpected fake git command: $*" >&2
        exit 1
        ;;
    esac
    ;;
  docker)
    if [[ "${1:-}" == "pull" ]]; then
      image=${2:-}
      if [[ "${FAIL_PULL:-0}" == 1 && "$image" == *":sha-${DEPLOY_SHA}" ]]; then
        exit 1
      fi
      exit 0
    fi
    if [[ "${1:-}" == "inspect" ]]; then
      if [[ "$*" == *".Config.Image"* ]]; then
        printf 'ghcr.io/itsthompson/gofin/expense-service:sha-%s\n' "$PREVIOUS_SHA"
      else
        printf 'old-image-id\n'
      fi
      exit 0
    fi
    if [[ "${1:-}" == "image" && "${2:-}" == "inspect" ]]; then
      if [[ "$*" == *":sha-${DEPLOY_SHA}"* ]]; then
        printf 'new-image-id\n'
      else
        printf 'old-image-id\n'
      fi
      exit 0
    fi
    if [[ "${1:-}" != "compose" ]]; then
      echo "unexpected fake docker command: $*" >&2
      exit 1
    fi
    if [[ "$*" == *"config --format json"* ]]; then
      cat "$COMPOSE_CONFIG_FILE"
      exit 0
    fi
    if [[ "$*" == *"config --services"* ]]; then
      printf 'expense-service\n'
      exit 0
    fi
    if [[ "$*" == *"ps -q"* ]]; then
      printf 'old-container\n'
      exit 0
    fi
    if [[ "$*" == *"ps -a --format json"* ]]; then
      printf '%s\n' "$*" >>"$FAKE_EVENTS_FILE"
      if [[ "${FAIL_HEALTH:-0}" == 1 && "$*" != *"gofin-rollback"* ]]; then
        printf '[{"State":"running","Health":"unhealthy"}]\n'
      else
        printf '[{"State":"running","Health":"healthy"}]\n'
      fi
      exit 0
    fi
    if [[ "$*" == *"exec -T auth-service"* ]]; then
      printf '%s\n' "$*" >>"$FAKE_EVENTS_FILE"
      [[ "${FAIL_SEED:-0}" != 1 ]]
      exit $?
    fi
    if [[ "$*" == *" up "* ]]; then
      printf '%s\n' "$*" >>"$FAKE_EVENTS_FILE"
      [[ "${FAIL_START:-0}" != 1 || "$*" == *"gofin-rollback"* ]]
      exit $?
    fi
    echo "unexpected fake docker compose command: $*" >&2
    exit 1
    ;;
  envsubst)
    cat
    ;;
  sleep)
    exit 0
    ;;
  mv)
    if [[ "${FAIL_MARKER:-0}" == 1 && "${3:-}" == "${DEPLOY_ROOT}/.deployed-sha" && ! -e "$FAKE_MARKER_MOVED" ]]; then
      : >"$FAKE_MARKER_MOVED"
      exit 1
    fi
    exec "$REAL_MV" "$@"
    ;;
  *)
    echo "unexpected fixture command name: $command_name" >&2
    exit 1
    ;;
esac
