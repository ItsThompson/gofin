# Root justfile: orchestrates the full gofin monorepo

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

# Start a specific backend service for development
dev-service service:
    cd services/{{service}} && go run ./cmd/main.go

# Start infrastructure only (databases + monitoring)
dev-infra:
    docker compose up -d postgresql immudb prometheus alertmanager grafana

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

# Run all tests
test: test-backend test-frontend

# Run database migrations
migrate service direction="up":
    cd services/{{service}} && \
    migrate -path db/migrations \
    -database "$(if [ '{{service}}' = 'auth' ]; then echo $AUTH_DB_URL; else echo $FINANCE_DB_URL; fi)" \
    {{direction}}

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
