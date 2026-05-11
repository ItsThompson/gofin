# Root justfile: orchestrates the full gofin monorepo

# Install dev tooling and git hooks
setup:
    #!/usr/bin/env bash
    set -euo pipefail

    echo "Installing dev tools via Homebrew..."

    if ! command -v lefthook &> /dev/null; then
        brew install lefthook
    else
        echo "  lefthook already installed"
    fi

    if ! command -v golangci-lint &> /dev/null; then
        brew install golangci-lint
    else
        echo "  golangci-lint already installed"
    fi

    echo "Installing git hooks..."
    lefthook install

    echo "Done. Pre-commit hooks are active."

# Start the full stack
up:
    docker compose up -d --build

# Start the full stack with logs
up-logs:
    docker compose up --build

# Start the full stack with dev overrides
up-dev:
    docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

# Stop all containers
down:
    docker compose down

# Stop and remove volumes (full reset)
reset:
    docker compose down -v

# Start only the frontend for development (hot reload)
dev-frontend:
    cd frontend && npx turbo dev

# Start the frontend in mock mode (no backend required)
dev-mock:
    cd frontend/apps/shell && npm run dev:mock

# Start a specific backend service for development
dev-service service:
    cd services/{{service}} && MIGRATIONS_PATH=./db/migrations go run ./cmd/main.go

# Start infrastructure only (databases + monitoring)
dev-infra:
    docker compose up -d postgresql immudb prometheus alertmanager grafana cadvisor node-exporter

# Run all backend tests
test-backend:
    cd services && go work sync && \
    cd auth && go test ./... && \
    cd ../expense && go test ./... && \
    cd ../finance && go test ./... && \
    cd ../gateway && go test ./...

# Run frontend tests
test-frontend:
    cd frontend && npx turbo test

# Run E2E tests (requires full stack via `just up` and `just seed-admin`)
test-e2e:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Checking stack health..."
    if ! curl -sf -o /dev/null http://localhost:3000 2>/dev/null; then
        echo "ERROR: Frontend is not reachable at http://localhost:3000" >&2
        echo "Run 'just up' to start the full stack before running E2E tests." >&2
        exit 1
    fi
    if ! curl -sf -o /dev/null http://localhost:3000/api/health 2>/dev/null; then
        echo "ERROR: API gateway is not reachable at http://localhost:3000/api/health" >&2
        echo "Run 'just up' and wait for all services to be healthy." >&2
        exit 1
    fi
    echo "Stack is healthy. Running E2E tests..."
    cd e2e && npx playwright test

# Run all tests
test: test-backend test-frontend

# Create a new database migration file pair
migrate-create service name:
    migrate create -ext sql -dir services/{{service}}/db/migrations -seq {{name}}

# Roll back the last applied migration
migrate-down service:
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{service}}" in
      auth)       db_url="$AUTH_DB_URL" ;;
      finance)    db_url="$FINANCE_DB_URL" ;;
      datarights) db_url="$DATARIGHTS_DB_URL" ;;
      *) echo "Unknown service: {{service}}"; exit 1 ;;
    esac
    cd services/{{service}} && \
    migrate -path db/migrations \
    -database "${db_url}" \
    down 1

# Generate sqlc code
sqlc service:
    cd services/{{service}} && sqlc generate

# Generate protobuf code
proto service:
    cd services/{{service}} && \
    protoc \
    --go_out=. --go_opt=module=github.com/ItsThompson/gofin/services/{{service}} \
    --go-grpc_out=. --go-grpc_opt=module=github.com/ItsThompson/gofin/services/{{service}} \
    proto/*.proto

# Generate all protobuf code
proto-all:
    just proto auth
    just proto expense
    just proto finance

# Lint backend
lint-backend:
    cd services && golangci-lint run ./...

# Lint frontend
lint-frontend:
    cd frontend && npx turbo lint

# Lint all
lint: lint-backend lint-frontend

# Open Grafana in browser
grafana:
    open http://localhost:3001

# View logs for a specific service
logs service:
    docker compose logs -f {{service}}

# Seed the first admin user
seed-admin:
    @echo "Seeding admin user..."
    docker compose exec auth-service /service seed-admin
