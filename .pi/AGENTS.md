# GoFin Project

## Identity

- **Go module path**: `github.com/ItsThompson/gofin` (capital I, capital T, case-sensitive)
- **Git identity**: thompson / hi.thompson@hotmail.com
- **Remote**: `https://github.com/ItsThompson/gofin.git`

## Repository Structure

```
gofin/
├── services/          # Go microservices (auth, expense, finance, gateway, metrics)
│   └── go.work       # Go workspace file
├── frontend/          # Turborepo workspace (shell, finance, admin + packages)
├── e2e/               # Standalone Playwright project
├── monitoring/        # Prometheus, Grafana, alert rules
├── deployments/       # Cloudflare tunnel configs
├── scripts/           # Deploy scripts
└── docker-compose.yml
```

## Gateway / Proxy Architecture

- Gateway uses **wildcard proxy** (`/*path`): new downstream endpoints need NO gateway router changes
- `http-proxy-middleware`: use `pathFilter` option, not Express path mounting (Express `app.use("/api", handler)` strips the prefix from `req.url`)
- `cookieDomainRewrite: ""` means "remove Domain attribute", not "don't rewrite": omit the option entirely to pass cookies through unchanged
- Gin: always set `RedirectTrailingSlash = false` AND register both `Any("")` and `Any("/*path")` on route groups (wildcard alone misses the exact group prefix)
- Strip identity headers (`X-User-ID`, `X-User-Role`, `X-Assumed-By`) unconditionally BEFORE any routing logic

## Docker / Deployment (Hetzner CX33: 4 vCPU, 8GB RAM)

- Sequential builds recommended for production: `just up-prod` (5 Go compilers + Node/Vite can exceed 8GB peak when parallel)
- `set -a` before `source .env` when sourced vars are consumed by subprocesses (envsubst, docker compose)
- Bind-mounted files must exist on the host before `docker compose up` (Docker creates directories for missing source paths)
- All containers must have explicit `deploy.resources.limits` for monitoring alerts to function correctly
- Config file deploys: compare-then-restart pattern to avoid unnecessary container churn from `systemctl restart docker`

## Packages / Dependencies

- Unified `radix-ui` package (not individual `@radix-ui/react-*`)
- shadcn style: `radix-nova` (not "new-york"), Tailwind v4 (CSS-based config, no `tailwind.config.ts`)
- Before upgrading any dependency in the monorepo: `grep -r '"<package>"' --include="package.json" frontend/`

## Period System

- Strictly calendar-month-based: `BudgetPeriod` has only `Year` + `Month` (no custom start/end dates)
- Dashboard endpoints use `?year=YYYY&month=MM` anchoring convention
