package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"api-gateway/internal/observability"
)

type retryTransport struct {
	inner     http.RoundTripper
	maxRetries int
	backoff    time.Duration
	logger     observability.Logger
}

func NewRetryTransport(inner http.RoundTripper, maxRetries int, backoff time.Duration, logger observability.Logger) http.RoundTripper {
	return &retryTransport{
		inner:      inner,
		maxRetries: maxRetries,
		backoff:    backoff,
		logger:     logger,
	}
}

func (rt *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error

	// Buffer the request body if it exists
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	for i := 0; i <= rt.maxRetries; i++ {
		if i > 0 {
			time.Sleep(rt.backoff * time.Duration(i))
			// Reset request body for retry
			if req.Body != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		resp, err := rt.inner.RoundTrip(req)
		if err != nil {
			lastErr = err
			rt.logger.Warn("Request failed, retrying", "attempt", i+1, "error", err)
			continue
		}

		// Check if response is retryable
		if resp.StatusCode >= 500 || resp.StatusCode == 408 || resp.StatusCode == 429 {
			lastResp = resp
			rt.logger.Warn("Retryable status code, retrying", "attempt", i+1, "status", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}
