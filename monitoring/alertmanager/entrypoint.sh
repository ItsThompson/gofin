#!/bin/sh
set -e

# Use sed for selective substitution: only replaces the literal string
# ${DISCORD_WEBHOOK_URL} with its value. This preserves Go template syntax
# (e.g., {{ .GroupLabels.alertname }}, {{ range .Alerts }}) which envsubst
# would clobber if the variable list filter were misconfigured.
sed "s|\${DISCORD_WEBHOOK_URL}|${DISCORD_WEBHOOK_URL}|g" \
  /etc/alertmanager/alertmanager.yml.tpl > /etc/alertmanager/alertmanager.yml

exec /bin/alertmanager "$@"
