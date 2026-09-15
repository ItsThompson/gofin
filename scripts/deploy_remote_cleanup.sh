#!/usr/bin/env bash
set -euo pipefail

if ! docker builder prune -af --filter "until=24h"; then
  echo "WARNING: builder prune failed (non-critical)" >&2
fi
if ! docker image prune -af --filter "until=72h"; then
  echo "WARNING: image prune failed (non-critical)" >&2
fi
journalctl --vacuum-size=50M 2>/dev/null || true
apt-get clean -y 2>/dev/null || true
: > /var/log/btmp 2>/dev/null || true
docker system df 2>/dev/null || true
