# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY reporting/go.mod ./reporting/
COPY auth/go.mod ./auth/
COPY datarights/go.mod ./datarights/
COPY expense/go.mod ./expense/
COPY finance/go.mod ./finance/
COPY shared/reporting/go.mod shared/reporting/go.sum* ./shared/reporting/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd reporting && GOWORK=off go mod download

COPY reporting/ ./reporting/
COPY auth/proto/ ./auth/proto/
COPY datarights/proto/ ./datarights/proto/
COPY expense/proto/ ./expense/proto/
COPY finance/proto/ ./finance/proto/
COPY shared/reporting/ ./shared/reporting/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd reporting && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /service ./cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
USER 65532:65532
ENTRYPOINT ["/service"]
