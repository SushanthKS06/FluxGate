# API Gateway

A production-grade API Gateway built with Go, featuring distributed rate limiting, JWT authentication, circuit breaker pattern, and comprehensive observability.

## Features

- **Reverse Proxy**: High-performance request forwarding with connection pooling
- **Distributed Rate Limiting**: Token bucket algorithm using Redis
- **JWT Authentication**: Secure token validation middleware
- **Load Balancing**: Round-robin distribution across upstreams
- **Circuit Breaker**: Distributed failure protection with Redis state
- **Retry Mechanism**: Exponential backoff for transient failures
- **Observability**: Structured JSON logging, Prometheus metrics, request tracing
- **Security**: TLS support, security headers, input validation

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         API Gateway                              │
├─────────────────────────────────────────────────────────────────┤
│  Middleware Chain:                                              │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │ Request ID  │ │  Logging    │ │Rate Limit   │               │
│  └─────────────┘ └─────────────┘ └─────────────┘               │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │    JWT      │ │Circuit Brk  │ │  Timeout    │               │
│  └─────────────┘ └─────────────┘ └─────────────┘               │
│  ┌─────────────┐ ┌─────────────┐                               │
│  │   Retry     │ │Security Hdr │                               │
│  └─────────────┘ └─────────────┘                               │
├─────────────────────────────────────────────────────────────────┤
│  Load Balancer (Round-Robin)                                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                        │
│  │ Upstream1│ │ Upstream2│ │ Upstream3│                        │
│  └──────────┘ └──────────┘ └──────────┘                        │
├─────────────────────────────────────────────────────────────────┤
│  Dependencies:                                                  │
│  ┌──────────┐ ┌──────────┐                                     │
│  │  Redis   │ │Prometheus│                                     │
│  └──────────┘ └──────────┘                                     │
└─────────────────────────────────────────────────────────────────┘
```

## Request Flow

```
Client Request
    │
    ▼
┌─────────────────┐
│ 1. Request ID   │ ─── Generate unique ID, add to context
└────────┬────────┘
         │
    ┌────▼────────┐
│ 2. Logging     │ ─── Start timer, log request start
└────┬───────────┘
         │
    ┌────▼────────┐
│ 3. Rate Limit  │ ─── Check Redis token bucket, allow/block
└────┬───────────┘
         │
    ┌────▼────────┐
│ 4. JWT Auth    │ ─── Validate Bearer token
└────┬───────────┘
         │
    ┌────▼────────┐
│ 5. Circuit Brk │ ─── Check state, fail fast if open
└────┬───────────┘
         │
    ┌────▼────────┐
│ 6. Timeout     │ ─── Apply request timeout
└────┬───────────┘
         │
    ┌────▼────────┐
│ 7. Retry       │ ─── Retry on 5xx with backoff
└────┬───────────┘
         │
    ┌────▼────────┐
│ 8. Security    │ ─── Add security headers
└────┬───────────┘
         │
    ┌────▼────────┐
│ 9. Load Balance│ ─── Round-robin upstream selection
└────┬───────────┘
         │
    ┌────▼────────┐
│10. Proxy       │ ─── Forward with connection pooling
└────┬───────────┘
         │
    ┌────▼────────┐
│11. Response    │ ─── Return to client
└─────────────────┘
```

## Tradeoffs

| Feature | Choice | Tradeoff |
|---------|--------|----------|
| Rate Limiting | Redis | Adds ~1ms latency vs in-memory, but enables distributed limiting |
| Circuit Breaker | Redis State | Requires Redis, but works across multiple gateway instances |
| Load Balancing | Round-Robin | Simple, but doesn't consider backend load |
| Retry | Exponential Backoff | Prevents thundering herd, but adds latency |
| Connection Pool | 100 connections | Memory vs performance tradeoff |

## Failure Scenarios

### Upstream Timeout
- Middleware applies context timeout
- Circuit breaker records failure
- Returns 503 to client

### Rate Limit Exceeded
- Redis token bucket denies request
- Returns 429 with Retry-After header
- Logs warning with client IP

### Circuit Breaker Open
- All requests fail fast
- Returns 503 Service Unavailable
- Half-open after timeout, tests with limited requests

### Retry Storm Prevention
- Exponential backoff: 100ms → 200ms → 400ms
- Max 3 retries
- Only retries 5xx, 408, 429 errors

## Configuration

```yaml
port: "8080"
redis_addr: "redis:6379"
rate_limit_rpm: 60
jwt_secret: "your-secret"
request_timeout: "30s"
max_retries: 3
retry_backoff: "100ms"
circuit_breaker_threshold: 5
circuit_breaker_timeout: "10s"
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | Server port |
| REDIS_ADDR | localhost:6379 | Redis address |
| RATE_LIMIT_RPM | 60 | Requests per minute |
| JWT_SECRET | - | JWT signing secret |
| REQUEST_TIMEOUT | 30s | Request timeout |
| MAX_RETRIES | 3 | Max retry attempts |
| RETRY_BACKOFF | 100ms | Base backoff duration |
| CIRCUIT_BREAKER_THRESHOLD | 5 | Failures before open |
| CIRCUIT_BREAKER_TIMEOUT | 10s | Time before half-open |
| TLS_CERT | - | TLS certificate path |
| TLS_KEY | - | TLS key path |

## Deployment

### Docker

```bash
docker build -f docker/Dockerfile -t api-gateway .
docker run -p 8080:8080 -e REDIS_ADDR=redis:6379 api-gateway
```

### Kubernetes

```bash
kubectl apply -f k8s/deployment.yaml
```

## Load Testing

```bash
k6 run loadtest/loadtest.js
```

## Metrics

Prometheus metrics available at `/metrics`:

- `api_gateway_request_duration_seconds` - Request latency histogram
- `api_gateway_requests_total` - Total requests counter
- `api_gateway_rate_limit_hits_total` - Rate limit hits
- `api_gateway_circuit_breaker_state` - Circuit breaker state
- `api_gateway_upstream_health` - Upstream health status

## Health Check

```bash
curl http://localhost:8080/health
```

## Interview Questions

### Why Redis over in-memory rate limiting?
Redis enables distributed rate limiting across multiple gateway instances. In-memory would require sticky sessions or complex synchronization, introducing single points of failure. Redis provides atomic Lua scripts ensuring consistency under high concurrency.

### How token bucket works internally?
Token bucket allows burst traffic while enforcing average rate. Tokens are added at fixed rate (tokens/second). Requests consume tokens; if bucket empty, request is denied. Implementation tracks last refill time, calculates tokens to add based on elapsed time, ensures bucket never exceeds capacity.

### How would you scale to multiple instances?
Deploy behind load balancer (ALB, Traefik). Use Redis for shared state (rate limits, circuit breaker). Implement service registry (Consul, etcd) for upstream discovery. Add distributed tracing (OpenTelemetry) for observability. Containerize with Kubernetes for horizontal scaling.

## Project Structure

```
api-gateway/
├── cmd/gateway/main.go          # Entry point
├── docker/Dockerfile            # Container build
├── internal/
│   ├── config/config.go         # Configuration
│   ├── middleware/middleware.go # Middleware chain
│   ├── proxy/proxy.go           # Reverse proxy
│   ├── loadbalancer/roundrobin.go
│   ├── ratelimit/tokenbucket.go
│   └── observability/
├── k8s/                         # Kubernetes manifests
├── loadtest/                    # k6 load tests
├── tests/                       # Unit tests
└── README.md
```

## License

MIT
