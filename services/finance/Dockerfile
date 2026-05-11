# Stage 1: Build
# Context is ./services (set in docker-compose.yml) so the metrics and
# expense modules are available for the replace directives in go.mod.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files for dependency caching.
COPY finance/go.mod finance/go.sum* ./finance/
COPY dbmigrate/go.mod dbmigrate/go.sum* ./dbmigrate/
COPY metrics/go.mod metrics/go.sum* ./metrics/
COPY expense/go.mod expense/go.sum* ./expense/
RUN cd finance && GOWORK=off go mod download

# Copy source.
COPY finance/ ./finance/
COPY dbmigrate/ ./dbmigrate/
COPY metrics/ ./metrics/
COPY expense/ ./expense/

RUN cd finance && GOWORK=off CGO_ENABLED=0 go build -o /service ./cmd/main.go

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service
COPY finance/db/migrations /migrations
EXPOSE 8083 9083
CMD ["/service"]
