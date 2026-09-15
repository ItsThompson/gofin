#!/usr/bin/env bash
set -euo pipefail

marker=/opt/gofin/.deployed-sha
if [[ ! -f "$marker" ]]; then
  exit 0
fi
previous_sha="$(<"$marker")"
marker_bytes="$(wc -c <"$marker")"
if [[ ! "$previous_sha" =~ ^[0-9a-f]{40}$ || ( "$marker_bytes" -ne "${#previous_sha}" && "$marker_bytes" -ne $(( ${#previous_sha} + 1 )) ) ]]; then
  echo "ERROR: existing .deployed-sha must contain exactly one SHA line" >&2
  exit 1
fi
printf '%s\n' "$previous_sha"
