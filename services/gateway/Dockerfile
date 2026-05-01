# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN [ -f go.sum ] && go mod download || true
COPY . .
RUN CGO_ENABLED=0 go build -o /service ./cmd/

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service
EXPOSE 8080
CMD ["/service"]
