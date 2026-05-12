# syntax=docker/dockerfile:1

# Stage 1: Build
# Context is ./services (set in docker-compose.yml) so the metrics module
# is available for the replace directive in go.mod.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files for dependency caching.
COPY expense/go.mod expense/go.sum* ./expense/
COPY healthcheck/go.mod ./healthcheck/
COPY metrics/go.mod metrics/go.sum* ./metrics/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd expense && GOWORK=off go mod download

# Copy source.
COPY expense/ ./expense/
COPY healthcheck/ ./healthcheck/
COPY metrics/ ./metrics/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd expense && GOWORK=off CGO_ENABLED=0 go build -tags docker -o /service ./cmd/

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service
EXPOSE 8082 9082
CMD ["/service"]
