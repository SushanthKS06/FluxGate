package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_gateway_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_gateway_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	rateLimitHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_gateway_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"client_ip"},
	)

	circuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "api_gateway_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"upstream"},
	)

	upstreamHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "api_gateway_upstream_health",
			Help: "Upstream health status (1=healthy, 0=unhealthy)",
		},
		[]string{"upstream"},
	)
)

func init() {
	prometheus.MustRegister(requestDuration, requestsTotal, rateLimitHits, circuitBreakerState, upstreamHealth)
}

func RecordRequestDuration(method, path string, status int, duration time.Duration) {
	requestDuration.WithLabelValues(method, path, http.StatusText(status)).Observe(duration.Seconds())
}

func RecordRequestTotal(method, path string, status int) {
	requestsTotal.WithLabelValues(method, path, http.StatusText(status)).Inc()
}

func RecordRateLimitHit(clientIP string) {
	rateLimitHits.WithLabelValues(clientIP).Inc()
}

func SetCircuitBreakerState(upstream string, state int) {
	circuitBreakerState.WithLabelValues(upstream).Set(float64(state))
}

func SetUpstreamHealth(upstream string, healthy bool) {
	val := 0.0
	if healthy {
		val = 1.0
	}
	upstreamHealth.WithLabelValues(upstream).Set(val)
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
