# Stage 1: Build
# Context is ./services (set in docker-compose.yml) so the metrics module
# is available for the replace directive in go.mod.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files for dependency caching.
COPY datarights/go.mod datarights/go.sum* ./datarights/
COPY auth/go.mod auth/go.sum* ./auth/
COPY expense/go.mod expense/go.sum* ./expense/
COPY finance/go.mod finance/go.sum* ./finance/
COPY metrics/go.mod metrics/go.sum* ./metrics/
RUN cd datarights && GOWORK=off go mod download

# Copy source.
COPY datarights/ ./datarights/
COPY auth/ ./auth/
COPY expense/ ./expense/
COPY finance/ ./finance/
COPY metrics/ ./metrics/

RUN cd datarights && GOWORK=off CGO_ENABLED=0 go build -o /service ./cmd/main.go

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service
COPY datarights/db/migrations /migrations
EXPOSE 8084 9084
CMD ["/service"]
