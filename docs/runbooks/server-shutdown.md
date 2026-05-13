# Server Shutdown & Restart

Safe procedure for powering off and restarting the Hetzner VPS without data loss.

## Why This Matters

Hetzner's console "Power Off" button is equivalent to pulling the power plug. It does not send a graceful shutdown signal to the OS. Databases (PostgreSQL, immudb) may have dirty pages in memory that haven't been flushed to disk, which can cause corruption.

## Recommended: Simple Safe Shutdown (Zero Manual Steps on Reboot)

SSH into the server and run:

```bash
ssh root@<server-ip>
cd /opt/gofin

# 1. Flush PostgreSQL WAL and dirty pages to disk
docker compose exec postgresql psql -U gofin -d gofin -c "CHECKPOINT;"

# 2. Power off the OS cleanly
shutdown -h now
```

That's it. On reboot, everything comes back automatically:

- `shutdown -h now` sends SIGTERM to dockerd, which propagates SIGTERM to all containers (10s grace period)
- All services have `restart: unless-stopped`, so Docker auto-starts them on boot
- Cloudflare tunnels reconnect automatically (no re-authentication needed)
- Database volumes (`pgdata`, `immudata`, `promdata`) persist on disk
- Rendered tunnel configs and credentials persist on the host filesystem
- Admin user and all data survive (stored in named volumes)

### One-liner version

```bash
ssh root@<server-ip> "cd /opt/gofin && docker compose exec postgresql psql -U gofin -d gofin -c 'CHECKPOINT;' && shutdown -h now"
```

### Why this works

| Step | What happens |
|------|--------------|
| `CHECKPOINT` | Forces PostgreSQL to write all in-memory dirty pages and WAL to disk |
| `shutdown -h now` | Clean OS shutdown: sends SIGTERM to dockerd, dockerd sends SIGTERM to containers, containers shut down gracefully, filesystem buffers flush, volumes unmount |
| Reboot | Docker daemon starts (systemd enabled), restarts all `unless-stopped` containers in dependency order |

## Alternative: Explicit Shutdown (Requires Manual Restart)

Use this if you want full control over container lifecycle or plan to change configuration before restarting:

```bash
ssh root@<server-ip>
cd /opt/gofin

# 1. Flush PostgreSQL
docker compose exec postgresql psql -U gofin -d gofin -c "CHECKPOINT;"

# 2. Remove all containers explicitly (including tunnel profile)
docker compose --profile tunnels down

# 3. Power off
shutdown -h now
```

**After reboot, you must manually start the stack:**

```bash
ssh root@<server-ip>
cd /opt/gofin
docker compose --profile tunnels up -d
```

### When to use explicit shutdown

- Changing `.env` variables before restart
- Updating tunnel config templates (need to re-render)
- Upgrading Docker or the host OS
- Debugging: want a clean slate without leftover containers

## Starting the Server Back Up

### After simple shutdown (auto-restart)

1. Power on via Hetzner Cloud Console (or `hcloud server poweron <name>`)
2. Wait ~30s for services to come up
3. Verify:

```bash
ssh root@<server-ip>
cd /opt/gofin

# All containers should be running
docker compose --profile tunnels ps

# API health
curl -s http://localhost:8080/health

# PostgreSQL ready
docker compose exec postgresql pg_isready -U gofin
```

### After explicit shutdown (manual restart)

1. Power on via Hetzner Cloud Console
2. SSH in and start:

```bash
ssh root@<server-ip>
cd /opt/gofin
docker compose --profile tunnels up -d
```

3. Verify with the same commands above.

### Startup timing notes

- PostgreSQL has a healthcheck (`pg_isready`). Services with `condition: service_healthy` wait for it.
- immudb has no healthcheck. The expense-service uses `condition: service_started` and retries connections internally. It may take a few extra seconds on first boot.
- Cloudflare tunnels reconnect within seconds once their upstream services (mfe) are listening.

## Emergency: Hetzner Console Power Off Was Used (or Crash)

If the server was hard-powered-off without a clean shutdown:

```bash
ssh root@<server-ip>
cd /opt/gofin

# Containers should auto-restart via unless-stopped policy.
# If tunnels aren't running (they should be), start them:
docker compose --profile tunnels up -d

# Check PostgreSQL crash recovery logs
docker compose logs postgresql 2>&1 | grep -i "recovery\|redo\|corrupt\|invalid"

# Verify data integrity
docker compose exec postgresql psql -U gofin -d gofin -c "SELECT count(*) FROM auth.users;"
docker compose exec postgresql psql -U gofin -d gofin -c "SELECT count(*) FROM finance.budget_periods;"

# Check immudb started cleanly (index rebuild may take a moment)
docker compose logs expense-service 2>&1 | grep -i "connect\|error\|retry"
```

### What crash recovery looks like

**PostgreSQL:** WAL (write-ahead logging) replays uncommitted transactions on startup. You'll see log lines like `redo starts at ...` and `redo done at ...`. This is normal and means recovery succeeded.

**immudb:** With `synced=true` (default), committed data is safe. The index may rebuild on startup, which can add a few seconds before the expense-service connects.

**Prometheus:** May lose the last few scrape intervals (seconds to minutes). Not a data integrity concern.

## Prerequisite: Docker Daemon Enabled at Boot

Verify this once on your server:

```bash
systemctl is-enabled docker
# Should output: enabled

# If not:
systemctl enable docker
```

## Cloudflare Tunnels

No Cloudflare setup is needed on restart. Here's why:

- Rendered configs (`config-app.rendered.yml`, `config-grafana.rendered.yml`) are regular files on the host filesystem that persist across reboots
- Credentials (`.json` files, `cert.pem`) also persist on the host
- DNS CNAME records point to the tunnel UUID, not the server IP: no DNS changes needed
- cloudflared reconnects to Cloudflare's edge automatically using existing credentials

The only time you'd need to redo Cloudflare setup:
- Server was wiped / OS reinstalled
- Tunnel credentials revoked in Cloudflare dashboard
- Changing domains or tunnel IDs (edit `.env` and re-render configs)

## Decision Flowchart

```
Need to power off?
│
├─ Just a reboot, no config changes?
│  └─ CHECKPOINT → shutdown -h now
│     └─ On boot: everything auto-restarts. Done.
│
├─ Changing .env or tunnel config?
│  └─ CHECKPOINT → docker compose --profile tunnels down → shutdown -h now
│     └─ On boot: re-render configs if needed, then docker compose --profile tunnels up -d
│
└─ Server crashed / hard power-off happened?
   └─ Power on → SSH in → check logs for recovery messages → verify data counts
```
