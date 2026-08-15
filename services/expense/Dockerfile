# syntax=docker/dockerfile:1

# Stage 1: Build
# Context is ./services (set in docker-compose.yml) so the metrics module
# is available for the replace directive in go.mod.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files for dependency caching.
COPY expense/go.mod expense/go.sum* ./expense/
COPY access/go.mod access/go.sum* ./access/
COPY healthcheck/go.mod ./healthcheck/
COPY metrics/go.mod metrics/go.sum* ./metrics/
COPY serverkit/go.mod serverkit/go.sum* ./serverkit/
COPY dbmigrate/go.mod dbmigrate/go.sum* ./dbmigrate/
COPY apierr/go.mod apierr/go.sum* ./apierr/
COPY errkit/go.mod errkit/go.sum* ./errkit/
COPY httpx/go.mod httpx/go.sum* ./httpx/
COPY finance/go.mod finance/go.sum* ./finance/
COPY fx/go.mod fx/go.sum* ./fx/
COPY shared/currency/go.mod ./shared/currency/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd expense && GOWORK=off go mod download

# Copy source.
COPY expense/ ./expense/
COPY access/ ./access/
COPY healthcheck/ ./healthcheck/
COPY metrics/ ./metrics/
COPY serverkit/ ./serverkit/
COPY dbmigrate/ ./dbmigrate/
COPY apierr/ ./apierr/
COPY errkit/ ./errkit/
COPY httpx/ ./httpx/
COPY finance/proto/ ./finance/proto/
COPY fx/proto/ ./fx/proto/
COPY shared/currency/ ./shared/currency/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd expense && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -tags docker -o /service ./cmd/

# Stage 2: Runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE 8082 9082
USER 65532:65532
ENTRYPOINT ["/service"]
