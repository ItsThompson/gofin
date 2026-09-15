#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" != "compose" ]]; then
  echo "unexpected docker arguments" >&2
  exit 1
fi
cat "${COMPOSE_FIXTURE:?COMPOSE_FIXTURE is required}"
