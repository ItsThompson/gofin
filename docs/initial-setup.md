# Cloudflare Tunnel Setup

One-time interactive setup to create named Cloudflare tunnels for gofin. Run these steps on any machine with `cloudflared` installed and a browser available.

## Prerequisites

- A Cloudflare account with your domain added (free plan)
- `cloudflared` CLI installed: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/

## Steps

### 1. Authenticate

```bash
cloudflared tunnel login
```

This opens a browser. Select your domain and authorize.

### 2. Create tunnels

```bash
cloudflared tunnel create gofin-app
cloudflared tunnel create gofin-grafana
```

Each command prints a tunnel ID (UUID). Save these for step 4.

### 3. Route DNS

```bash
cloudflared tunnel route dns gofin-app usegofin.com
cloudflared tunnel route dns gofin-grafana grafana.usegofin.com
```

### 4. Copy credentials and certificate

The `tunnel create` commands saved credential JSON files to `~/.cloudflared/`. The `tunnel login` step saved `cert.pem`. Copy them into the project:

```bash
mkdir -p deployments/cloudflare
cp ~/.cloudflared/<app-tunnel-id>.json deployments/cloudflare/gofin-app.json
cp ~/.cloudflared/<grafana-tunnel-id>.json deployments/cloudflare/gofin-grafana.json
cp ~/.cloudflared/cert.pem deployments/cloudflare/cert.pem
chmod 644 deployments/cloudflare/*.json deployments/cloudflare/cert.pem
```

### 5. Configure .env

On the server, copy `.env.example` to `.env` and set production values. The critical ones to change from defaults:

```
# Generate a strong secret
JWT_SECRET=<output of: openssl rand -hex 32>
ENVIRONMENT=production

# Strong admin password
ADMIN_PASSWORD=<strong password>

# Enable HTTPS cookies (Cloudflare terminates TLS)
COOKIE_SECURE=true

# Enable subdomain cookie access for Grafana
COOKIE_DOMAIN=.usegofin.com

# Grafana auth proxy "Back to gofin" link
APP_URL=https://usegofin.com

# Tunnel IDs from step 2
CF_APP_TUNNEL_ID=<app-tunnel-id>
CF_APP_HOSTNAME=usegofin.com
CF_GRAFANA_TUNNEL_ID=<grafana-tunnel-id>
CF_GRAFANA_HOSTNAME=grafana.usegofin.com
```

All other values (database URLs, service addresses) can stay as defaults since services communicate via Docker networking.

## Result

After these steps you have:
- Two named tunnels registered with Cloudflare
- DNS CNAME records pointing your domain to the tunnels
- Credential JSON files and origin certificate ready for the deploy script to copy to the server
