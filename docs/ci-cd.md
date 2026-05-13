# CI/CD Pipeline

gofin uses GitHub Actions for continuous integration and deployment.

## Pipeline Overview

### CI (`.github/workflows/ci.yml`)

Runs on every push and pull request. Two phases:

1. **Lint** (parallel): `lint-backend` (golangci-lint) + `lint-frontend` (turbo lint)
2. **Test** (parallel, gated by lint): `test-backend` + `test-frontend` + `e2e`

If either lint job fails, all test jobs are skipped.

```
push/PR → lint-backend ──┐
                         ├─→ test-backend
push/PR → lint-frontend ─┤   test-frontend
                         └─→ e2e (Docker Compose)
```

### CD (`.github/workflows/cd.yml`)

Deploys to the VPS in one of two ways:

- **Automatic**: triggers after CI succeeds on a push to `main`
- **Manual**: triggered via the Actions UI (`workflow_dispatch`) by any collaborator with write access

The CD workflow decodes tunnel credentials from GitHub Secrets, sets up SSH, and runs `scripts/deploy.sh`.

## Required GitHub Secrets

All secrets must be configured in the repository settings under **Settings → Secrets and variables → Actions**.

| Secret | Contents | Encoding |
|--------|----------|----------|
| `DEPLOY_SSH_KEY` | Ed25519 private key for VPS access | Raw (paste the full key including headers) |
| `DEPLOY_SERVER_IP` | VPS IP address (e.g., `65.108.42.100`) | Raw string |
| `CF_APP_CREDENTIALS` | Contents of `deployments/cloudflare/gofin-app.json` | Base64 |
| `CF_GRAFANA_CREDENTIALS` | Contents of `deployments/cloudflare/gofin-grafana.json` | Base64 |
| `CF_CERT_PEM` | Contents of `deployments/cloudflare/cert.pem` | Base64 |

### Encoding Credentials

The credential files live on the server at `/opt/gofin/deployments/cloudflare/`. SSH in and base64-encode them:

```bash
ssh root@<your-server-ip>
cd /opt/gofin

base64 -w0 < deployments/cloudflare/gofin-app.json
base64 -w0 < deployments/cloudflare/gofin-grafana.json
base64 -w0 < deployments/cloudflare/cert.pem
```

Copy each output and paste it as the corresponding secret value in the GitHub UI (**Settings → Secrets and variables → Actions → New repository secret**).

### SSH Key Setup

1. Generate a dedicated deploy key (if you don't already have one):
   ```bash
   ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/gofin_deploy
   ```
2. Add the **public** key to the VPS:
   ```bash
   ssh-copy-id -i ~/.ssh/gofin_deploy.pub root@<server-ip>
   ```
3. Add the **private** key contents (`~/.ssh/gofin_deploy`) as the `DEPLOY_SSH_KEY` secret in GitHub.

## E2E Environment in CI

The E2E job builds and runs the full Docker Compose stack (without the `tunnels` profile):

1. Copies `.env.example` to `.env` (default values are sufficient for CI)
2. Runs `docker compose up -d --build`
3. Waits for `http://localhost:3000` (frontend) and `http://localhost:3000/api/health` (API gateway) with retry loops
4. Seeds the admin user via `docker compose exec -T auth-service /service seed-admin`
5. Runs Playwright against `http://localhost:3000` using `.env.test.example` values
6. Uploads `playwright-report/` as an artifact on failure

The health check retries are important because immudb has no explicit healthcheck and the expense-service may take additional time to connect on first start.

## Manual Deployment

To deploy manually without pushing code:

1. Go to **Actions → CD → Run workflow**
2. Select the `main` branch
3. Click **Run workflow**

This skips CI and deploys the current state of `main` directly.

## Branch Protection

To enforce that PRs cannot merge with failing CI, configure branch protection rules:

1. Go to **Settings → Branches → Add rule**
2. Branch name pattern: `main`
3. Enable: **Require status checks to pass before merging**
4. Add required checks: `lint-backend`, `lint-frontend`, `test-backend`, `test-frontend`, `e2e`
5. Enable: **Require branches to be up to date before merging**

## Concurrency

CI uses concurrency groups (`ci-${{ github.ref }}`) with `cancel-in-progress: true`. If you push multiple commits rapidly to the same branch, only the latest run continues. Older runs are cancelled to save runner time.

## Troubleshooting

### CD fails with "credential file is empty"

The base64-encoded secrets are likely misconfigured. Re-encode the files using the commands above and update the secrets in the GitHub UI.

### E2E times out waiting for services

Check the Docker Compose logs in the failed job's output. Common causes:
- PostgreSQL initialization taking longer than expected on the runner
- immudb failing to start (check for port conflicts)
- A service crashing on startup (missing environment variable)

### golangci-lint fails with unknown linter

The CI uses the default golangci-lint configuration. If you add a `.golangci.yml` to `services/`, ensure all referenced linters are available in the version installed by `golangci/golangci-lint-action`.
