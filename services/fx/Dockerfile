# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY fx/go.mod fx/go.sum* ./fx/
COPY apierr/go.mod ./apierr/
COPY dbmigrate/go.mod dbmigrate/go.sum* ./dbmigrate/
COPY errkit/go.mod errkit/go.sum* ./errkit/
COPY healthcheck/go.mod ./healthcheck/
COPY metrics/go.mod metrics/go.sum* ./metrics/
COPY shared/currency/go.mod ./shared/currency/
COPY serverkit/go.mod serverkit/go.sum* ./serverkit/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd fx && GOWORK=off go mod download

COPY fx/ ./fx/
COPY apierr/ ./apierr/
COPY dbmigrate/ ./dbmigrate/
COPY errkit/ ./errkit/
COPY healthcheck/ ./healthcheck/
COPY metrics/ ./metrics/
COPY shared/currency/ ./shared/currency/
COPY serverkit/ ./serverkit/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd fx && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /service ./cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE 8085 9085
USER 65532:65532
ENTRYPOINT ["/service"]
