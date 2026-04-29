# Root justfile: orchestrates the full gofin monorepo

# Start the full stack
up:
    echo "TODO: docker compose up -d --build"

# Stop all containers
down:
    echo "TODO: docker compose down"

# Stop and remove volumes (full reset)
reset:
    echo "TODO: docker compose down -v"

# Start only the frontend for development (hot reload)
dev-frontend:
    echo "TODO: cd frontend && npx turbo dev"

# Start a specific backend service for development
dev-service service:
    echo "TODO: cd services/{{service}} && go run ./cmd/main.go"

# Start infrastructure only (databases + monitoring)
dev-infra:
    echo "TODO: docker compose up -d postgresql immudb prometheus alertmanager grafana"

# Run all backend tests
test-backend:
    echo "TODO: cd services && go test ./..."

# Run frontend tests
test-frontend:
    echo "TODO: cd frontend && npx turbo test"

# Run all tests
test: test-backend test-frontend

# Lint all code
lint:
    echo "TODO: lint backend and frontend"
