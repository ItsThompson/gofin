#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
FIXTURE_COMMAND="${SCRIPT_DIR}/deploy_remote_fixture_command.sh"
DEPLOY_SHA=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
fake_bin=$(mktemp -d)
fixture_dir=$(mktemp -d)
credential_dir="${REPO_ROOT}/deployments/cloudflare"
cleanup() {
  local filename
  for filename in gofin-app.json gofin-grafana.json cert.pem; do
    if [[ -f "$fixture_dir/original-$filename" ]]; then
      cp -p "$fixture_dir/original-$filename" "$credential_dir/$filename"
    else
      rm -f "$credential_dir/$filename"
    fi
  done
  rm -rf "$fake_bin" "$fixture_dir"
}
trap cleanup EXIT
mkdir -p "$fixture_dir"
for filename in gofin-app.json gofin-grafana.json cert.pem; do
  if [[ -f "$credential_dir/$filename" ]]; then
    cp -p "$credential_dir/$filename" "$fixture_dir/original-$filename"
  fi
done

cp "$FIXTURE_COMMAND" "$fake_bin/ssh"
cp "$FIXTURE_COMMAND" "$fake_bin/scp"
chmod +x "$fake_bin"/*
mkdir -p "$credential_dir"
cp "${credential_dir}/config-app.yml" "${credential_dir}/gofin-app.json"
cp "${credential_dir}/config-grafana.yml" "${credential_dir}/gofin-grafana.json"
cp "${credential_dir}/config.yml" "${credential_dir}/cert.pem"

for failure_number in 1 2 3; do
  rm -f "$fixture_dir/ssh-events" "$fixture_dir/scp-count"
  set +e
  env \
    DEPLOY_SHA="$DEPLOY_SHA" \
    FAKE_SSH_EVENTS_FILE="$fixture_dir/ssh-events" \
    FAKE_SCP_COUNT_FILE="$fixture_dir/scp-count" \
    FAIL_SCP_NUMBER="$failure_number" \
    PATH="$fake_bin:$PATH" \
    "${SCRIPT_DIR}/deploy.sh" 127.0.0.1 >"$fixture_dir/stdout" 2>"$fixture_dir/stderr"
  status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    echo "Expected credential copy ${failure_number} to fail" >&2
    exit 1
  fi
  if [[ ! -f "$fixture_dir/ssh-events" ]]; then
    echo "SSH cleanup events missing for copy ${failure_number}" >&2
    exit 1
  fi
  cleanup_events=$(<"$fixture_dir/ssh-events")
  for path in \
    "/tmp/gofin-app-${DEPLOY_SHA}.json" \
    "/tmp/gofin-grafana-${DEPLOY_SHA}.json" \
    "/tmp/gofin-cert-${DEPLOY_SHA}.pem"; do
    if [[ "$cleanup_events" != *"$path"* ]]; then
      echo "Staged credential path was not cleaned after copy ${failure_number}: $path" >&2
      exit 1
    fi
  done
done

echo "deploy SCP cleanup fixture: all tests passed"
