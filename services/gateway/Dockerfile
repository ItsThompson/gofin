# Stage 1: Build
# Context is ./services (set in docker-compose.yml) so the auth module
# is available for the replace directive in go.mod.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files for dependency caching.
COPY gateway/go.mod gateway/go.sum* ./gateway/
COPY auth/go.mod auth/go.sum* ./auth/
RUN cd gateway && GOWORK=off go mod download

# Copy only what the gateway build needs: gateway source + auth proto.
COPY gateway/ ./gateway/
COPY auth/proto/ ./auth/proto/

RUN cd gateway && GOWORK=off CGO_ENABLED=0 go build -o /service ./cmd/

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service
EXPOSE 8080
CMD ["/service"]
