#!/bin/sh
set -e

# Substitute environment variables in the alertmanager config template
envsubst < /etc/alertmanager/alertmanager.yml.tpl > /etc/alertmanager/alertmanager.yml

exec /bin/alertmanager "$@"
