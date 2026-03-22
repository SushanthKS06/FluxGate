package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"api-gateway/internal/config"
	"api-gateway/internal/observability"
	"api-gateway/internal/ratelimit"
)

type Middleware func(http.Handler) http.Handler

func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Request ID middleware
func RequestIDMiddleware(logger observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := generateRequestID()
			ctx := context.WithValue(r.Context(), "requestID", requestID)
			r = r.WithContext(ctx)

			w.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(w, r)
		})
	}
}

// Logging middleware
func LoggingMiddleware(logger observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)

			requestID := ""
			if id := r.Context().Value("requestID"); id != nil {
				requestID = id.(string)
			}
			logger.Info("Request completed",
				"requestID", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"duration", duration.Milliseconds(),
				"status", "200",
			)
		})
	}
}

// Rate limiting middleware
func RateLimitMiddleware(client *redis.Client, rate int, logger observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Context().Value("requestID").(string)
			clientIP := getClientIP(r)
			tb := ratelimit.NewTokenBucket(client, clientIP, rate)

			allowed, err := tb.Allow(r.Context())
			if err != nil {
				logger.Error("Rate limit check failed", "requestID", requestID, "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				logger.Warn("Rate limit exceeded", "requestID", requestID, "clientIP", clientIP)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// JWT authentication middleware
func JWTMiddleware(secret string, logger observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Context().Value("requestID").(string)
			tokenString := r.Header.Get("Authorization")
			if tokenString == "" {
				logger.Warn("Missing authorization header", "requestID", requestID)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Assuming Bearer token
			if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
				logger.Warn("Invalid token format", "requestID", requestID)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			tokenString = tokenString[7:]

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				logger.Warn("Invalid JWT token", "requestID", requestID, "error", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Circuit breaker middleware
type CircuitBreaker struct {
	mu                sync.RWMutex
	failures          int
	state             string // "closed", "open", "half-open"
	lastFailureTime   time.Time
	threshold         int
	timeout           time.Duration
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     "closed",
		threshold: threshold,
		timeout:   timeout,
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "open" {
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = "half-open"
		} else {
			return http.ErrHandlerTimeout // or custom error
		}
	}

	err := fn()
	if err != nil {
		cb.failures++
		cb.lastFailureTime = time.Now()
		if cb.failures >= cb.threshold {
			cb.state = "open"
		}
		return err
	}

	if cb.state == "half-open" {
		cb.state = "closed"
		cb.failures = 0
	}
	return nil
}

func CircuitBreakerMiddleware(cfg *config.Config) Middleware {
	cb := NewCircuitBreaker(cfg.CircuitBreakerThreshold, cfg.CircuitBreakerTimeout)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := cb.Call(func() error {
				next.ServeHTTP(w, r)
				return nil // Assume success, in real impl check response status
			})
			if err != nil {
				http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			}
		})
	}
}

// Timeout middleware
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// Retry middleware
type responseWriter struct {
	http.ResponseWriter
	status int
	written bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.status = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.status = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(data)
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func RetryMiddleware(maxRetries int, backoff time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := wrapResponseWriter(w)
			var lastStatus int
			for i := 0; i <= maxRetries; i++ {
				if i > 0 {
					time.Sleep(backoff * time.Duration(i))
				}
				next.ServeHTTP(rw, r)
				lastStatus = rw.status
				if lastStatus < 500 {
					break
				}
			}
			// If final status is still 5xx, return error
			if lastStatus >= 500 {
				http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			}
		})
	}
}

// Metrics handler
func MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Placeholder for metrics
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Metrics endpoint"))
	})
}

// Redis client
func NewRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

// Utility functions
func generateRequestID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func getClientIP(r *http.Request) string {
	// Simplified, in real impl handle X-Forwarded-For etc.
	return r.RemoteAddr
}

// SecurityHeadersMiddleware adds security headers to responses
func SecurityHeadersMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			next.ServeHTTP(w, r)
		})
	}
}
