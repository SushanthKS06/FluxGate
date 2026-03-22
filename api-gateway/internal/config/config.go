package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                     string
	RedisAddr                string
	RateLimitRPM             int
	JWTSecret                string
	RequestTimeout           time.Duration
	MaxRetries               int
	RetryBackoff             time.Duration
	CircuitBreakerThreshold  int
	CircuitBreakerTimeout    time.Duration
	TLSCert                  string
	TLSKey                   string
	UpstreamHealthCheckInterval time.Duration
	EnableMetrics            bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                     getEnv("PORT", "8080"),
		RedisAddr:                getEnv("REDIS_ADDR", "localhost:6379"),
		RateLimitRPM:             getEnvInt("RATE_LIMIT_RPM", 60),
		JWTSecret:                getEnv("JWT_SECRET", "your-secret-key"),
		RequestTimeout:           getEnvDuration("REQUEST_TIMEOUT", 30*time.Second),
		MaxRetries:               getEnvInt("MAX_RETRIES", 3),
		RetryBackoff:             getEnvDuration("RETRY_BACKOFF", 100*time.Millisecond),
		CircuitBreakerThreshold:  getEnvInt("CIRCUIT_BREAKER_THRESHOLD", 5),
		CircuitBreakerTimeout:    getEnvDuration("CIRCUIT_BREAKER_TIMEOUT", 10*time.Second),
		TLSCert:                  getEnv("TLS_CERT", ""),
		TLSKey:                   getEnv("TLS_KEY", ""),
		UpstreamHealthCheckInterval: getEnvDuration("UPSTREAM_HEALTH_CHECK_INTERVAL", 30*time.Second),
		EnableMetrics:            getEnvBool("ENABLE_METRICS", true),
	}
	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
