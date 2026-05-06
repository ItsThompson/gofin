#!/bin/sh
set -e

# Substitute only DISCORD_WEBHOOK_URL to avoid clobbering Go template variables
envsubst '${DISCORD_WEBHOOK_URL}' < /etc/alertmanager/alertmanager.yml.tpl > /etc/alertmanager/alertmanager.yml

exec /bin/alertmanager "$@"
