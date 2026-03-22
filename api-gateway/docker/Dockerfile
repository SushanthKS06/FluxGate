FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o gateway ./cmd/gateway

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/gateway .
COPY --from=builder /app/config.yaml .

ENV PORT=8080 \
    REDIS_ADDR=redis:6379 \
    RATE_LIMIT_RPM=60 \
    JWT_SECRET=change-me-in-production \
    REQUEST_TIMEOUT=30s \
    MAX_RETRIES=3 \
    RETRY_BACKOFF=100ms \
    CIRCUIT_BREAKER_THRESHOLD=5 \
    CIRCUIT_BREAKER_TIMEOUT=10s

EXPOSE 8080

CMD ["./gateway"]
