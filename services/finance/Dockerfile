# syntax=docker/dockerfile:1

# Stage 1: Build
# Context is ./services (set in docker-compose.yml) so the metrics and
# expense modules are available for the replace directives in go.mod.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files for dependency caching.
COPY finance/go.mod finance/go.sum* ./finance/
COPY access/go.mod access/go.sum* ./access/
COPY dbmigrate/go.mod dbmigrate/go.sum* ./dbmigrate/
COPY healthcheck/go.mod ./healthcheck/
COPY metrics/go.mod metrics/go.sum* ./metrics/
COPY expense/go.mod expense/go.sum* ./expense/
COPY perf/go.mod ./perf/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd finance && GOWORK=off go mod download

# Copy source.
COPY finance/ ./finance/
COPY access/ ./access/
COPY dbmigrate/ ./dbmigrate/
COPY healthcheck/ ./healthcheck/
COPY metrics/ ./metrics/
COPY expense/ ./expense/
COPY perf/ ./perf/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd finance && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /service ./cmd/main.go

# Stage 2: Runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE 8083 9083
USER 65532:65532
ENTRYPOINT ["/service"]
