#!/usr/bin/env bash
set -euo pipefail

cat > /etc/cron.daily/docker-prune <<'EOF'
#!/bin/sh
# Remove all unused images, build cache, and containerd content older than 72h
echo "$(date): starting docker prune" >> /var/log/docker-prune.log
docker system prune -af --filter "until=72h" >> /var/log/docker-prune.log 2>&1
EOF
chmod +x /etc/cron.daily/docker-prune
# Remove old weekly cron if it exists
rm -f /etc/cron.weekly/docker-prune
echo "  Daily prune cron installed."
