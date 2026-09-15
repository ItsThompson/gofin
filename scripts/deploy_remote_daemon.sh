#!/usr/bin/env bash
set -euo pipefail

if ! cmp -s /tmp/daemon.json.new /etc/docker/daemon.json 2>/dev/null; then
  mv /tmp/daemon.json.new /etc/docker/daemon.json
  systemctl restart docker
  echo "  Docker daemon config updated and restarted."
else
  rm /tmp/daemon.json.new
  echo "  Docker daemon config unchanged, skipping restart."
fi
