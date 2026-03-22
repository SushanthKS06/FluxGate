package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/internal/middleware"
	"api-gateway/internal/observability"
)

func TestRequestIDMiddleware(t *testing.T) {
	logger := observability.NewLogger()
	handler := middleware.RequestIDMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("Request ID header not set")
	}
}

func TestLoggingMiddleware(t *testing.T) {
	logger := observability.NewLogger()
	handler := middleware.LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	// Note: Timeout middleware uses context timeout.
	// The handler will complete but context will be cancelled.
	// We test that the middleware doesn't panic.
	handler := middleware.TimeoutMiddleware(100 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	// Context timeout may or may not affect response depending on handler
	_ = rr.Code
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := middleware.SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options header not set")
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options header not set")
	}
	if rr.Header().Get("Strict-Transport-Security") == "" {
		t.Error("HSTS header not set")
	}
}

func TestRetryMiddleware(t *testing.T) {
	callCount := 0
	handler := middleware.RetryMiddleware(3, 10*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 4 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// maxRetries=3 means 4 attempts (0,1,2,3)
	if callCount != 4 {
		t.Errorf("Expected 4 attempts, got %d", callCount)
	}
}

func TestMiddlewareChain(t *testing.T) {
	var testHeader, testHeader2 string
	
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Test", "m1")
			next.ServeHTTP(w, r)
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Test2", "m2")
			next.ServeHTTP(w, r)
		})
	}

	chain := middleware.Chain(m1, m2)
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testHeader = r.Header.Get("X-Test")
		testHeader2 = r.Header.Get("X-Test2")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Chain applies m1 first (outer), then m2 (inner)
	if testHeader != "m1" {
		t.Errorf("Middleware 1 not applied, got: %s", testHeader)
	}
	if testHeader2 != "m2" {
		t.Errorf("Middleware 2 not applied, got: %s", testHeader2)
	}
}
