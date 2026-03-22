package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/middleware"
	"api-gateway/internal/observability"
	"api-gateway/internal/proxy"
)

func main() {
	// Initialize structured logging
	logger := observability.NewLogger()
	if zapLogger, ok := logger.(*observability.ZapLogger); ok {
		defer zapLogger.Sync()
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize Redis client for rate limiting and circuit breaker
	redisClient := middleware.NewRedisClient(cfg.RedisAddr)

	// Initialize upstream services for load balancing
	upstreams := []string{"http://upstream1:8080", "http://upstream2:8080", "http://upstream3:8080"}

	// Create reverse proxy with load balancing
	rp := proxy.NewReverseProxy(upstreams, cfg, logger)

	// Create middleware chain
	handler := middleware.Chain(
		middleware.RequestIDMiddleware(logger),
		middleware.LoggingMiddleware(logger),
		middleware.RateLimitMiddleware(redisClient, cfg.RateLimitRPM, logger),
		middleware.JWTMiddleware(cfg.JWTSecret, logger),
		middleware.CircuitBreakerMiddleware(cfg),
		middleware.TimeoutMiddleware(cfg.RequestTimeout),
		middleware.RetryMiddleware(cfg.MaxRetries, cfg.RetryBackoff),
		middleware.SecurityHeadersMiddleware(),
	)(rp)

	// Metrics endpoint
	http.Handle("/metrics", middleware.MetricsHandler())

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	// Main handler
	http.Handle("/", handler)

	// Create server with TLS if configured
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: nil,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		logger.Info("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server shutdown failed", "error", err)
		}
	}()

	logger.Info("Starting API Gateway", "port", cfg.Port)
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		if err := srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	} else {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}
}
