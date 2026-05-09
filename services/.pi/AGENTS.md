# GoFin Services (Go Backend)

## Commands

```bash
# Single service build/test:
cd services/<name> && GOWORK=off go build ./...
cd services/<name> && GOWORK=off go test ./... -count=1

# All services (from workspace root):
cd services && go build ./...
cd services && go test ./...

# sqlc code generation (no DB connection needed):
cd services/<name> && sqlc generate
```

## Rules

### JSON and API Contracts
- JSON struct tags must use **camelCase** matching TypeScript interfaces in `@gofin/types`
- Use `utf8.RuneCountInString()` not `len()` for VARCHAR length validation (Go `len()` counts bytes, PostgreSQL VARCHAR counts characters)

### Gin Request Binding
- Never use `binding:"required"` on numeric fields where zero is valid (Gin rejects zero values)
- Never use `binding:"required"` when service-layer validation exists (it blocks field-level error reporting with a generic "Invalid request body")
- `binding:"required"` is only for string fields where empty genuinely means "not provided"

### Go Modules and Workspace
- `go.work` masks `go.mod` insufficiency: always verify with `GOWORK=off go build ./...`
- When a replaced module's deps change, every consumer module (with `replace` directives pointing to it) needs `GOWORK=off go mod tidy`
- `go mod tidy` without build tags removes build-tag-gated deps: use `GOFLAGS="-tags=docker" go mod tidy` for the expense service

### Docker Builds
- Every Go service Dockerfile must use `context: ./services` (not individual service dir)
- Each `replace ... => ../X` in `go.mod` needs a matching `COPY X/ ./X/` in the Dockerfile
- Use `GOWORK=off` in Dockerfiles (workspace file isn't copied)

### Build Tags
- `//go:build docker` files (e.g., `immudb_prod.go`) only compile in Docker with `-tags docker`
- Local `go build` uses `!docker` stub implementations (no external SDK deps needed locally)

### sqlc
- `sqlc generate` works locally without a database (reads migration files as schema source)
- Expression-based UNIQUE constraints (`lower(name)`) fail sqlc parser: use `CREATE UNIQUE INDEX` instead

## Service Dependency Map

```
gateway → auth (proto), metrics
auth → metrics
expense → metrics
finance → metrics, expense (proto)
```
