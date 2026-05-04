# Stage 1: Build
# Context is ./services (set in docker-compose.yml) so the metrics module
# is available for the replace directive in go.mod.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files for dependency caching.
COPY expense/go.mod expense/go.sum* ./expense/
COPY metrics/go.mod metrics/go.sum* ./metrics/
RUN cd expense && GOWORK=off go mod download

# Copy source.
COPY expense/ ./expense/
COPY metrics/ ./metrics/

RUN cd expense && GOWORK=off CGO_ENABLED=0 go build -tags docker -o /service ./cmd/

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service
EXPOSE 8082 9082
CMD ["/service"]
