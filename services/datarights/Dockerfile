# syntax=docker/dockerfile:1

# Stage 1: Build
# Context is ./services (set in docker-compose.yml) so the metrics module
# is available for the replace directive in go.mod.
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy module files for dependency caching.
COPY datarights/go.mod datarights/go.sum* ./datarights/
COPY access/go.mod access/go.sum* ./access/
COPY auth/go.mod auth/go.sum* ./auth/
COPY dbmigrate/go.mod dbmigrate/go.sum* ./dbmigrate/
COPY expense/go.mod expense/go.sum* ./expense/
COPY finance/go.mod finance/go.sum* ./finance/
COPY healthcheck/go.mod ./healthcheck/
COPY metrics/go.mod metrics/go.sum* ./metrics/
COPY perf/go.mod ./perf/
COPY serverkit/go.mod serverkit/go.sum* ./serverkit/
COPY apierr/go.mod apierr/go.sum* ./apierr/
COPY errkit/go.mod errkit/go.sum* ./errkit/
COPY httpx/go.mod httpx/go.sum* ./httpx/
COPY pgutil/go.mod pgutil/go.sum* ./pgutil/
COPY shared/currency/go.mod ./shared/currency/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd datarights && GOWORK=off go mod download

# Copy source.
COPY datarights/ ./datarights/
COPY access/ ./access/
COPY auth/ ./auth/
COPY dbmigrate/ ./dbmigrate/
COPY expense/ ./expense/
COPY finance/ ./finance/
COPY healthcheck/ ./healthcheck/
COPY metrics/ ./metrics/
COPY perf/ ./perf/
COPY serverkit/ ./serverkit/
COPY apierr/ ./apierr/
COPY errkit/ ./errkit/
COPY httpx/ ./httpx/
COPY pgutil/ ./pgutil/
COPY shared/currency/ ./shared/currency/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd datarights && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /service ./cmd/main.go

# Stage 2: Runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE 8084 9084
USER 65532:65532
ENTRYPOINT ["/service"]
