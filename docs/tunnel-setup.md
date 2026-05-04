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

Add these to your production `.env` on the server:

```
CF_APP_TUNNEL_ID=<app-tunnel-id>
CF_APP_HOSTNAME=usegofin.com
CF_GRAFANA_TUNNEL_ID=<grafana-tunnel-id>
CF_GRAFANA_HOSTNAME=grafana.usegofin.com
```

## Result

After these steps you have:
- Two named tunnels registered with Cloudflare
- DNS CNAME records pointing your domain to the tunnels
- Credential JSON files and origin certificate ready for the deploy script to copy to the server
