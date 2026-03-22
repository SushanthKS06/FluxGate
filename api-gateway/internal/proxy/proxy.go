package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/loadbalancer"
	"api-gateway/internal/observability"
)

type ReverseProxy struct {
	lb       *loadbalancer.RoundRobin
	timeout  time.Duration
	logger   observability.Logger
	reverseProxy *httputil.ReverseProxy
}

func NewReverseProxy(upstreams []string, cfg *config.Config, logger observability.Logger) http.Handler {
	rp := &ReverseProxy{
		lb:     loadbalancer.NewRoundRobin(upstreams),
		timeout: cfg.RequestTimeout,
		logger: logger,
	}

	rp.reverseProxy = &httputil.ReverseProxy{
		Director: rp.director,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		ErrorHandler: rp.errorHandler,
	}

	return rp
}

func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rp.reverseProxy.ServeHTTP(w, r)
}

func (rp *ReverseProxy) director(req *http.Request) {
	targetURL := rp.lb.Next()
	url, _ := url.Parse(targetURL)

	req.URL.Scheme = url.Scheme
	req.URL.Host = url.Host
	req.Host = url.Host

	// Preserve original host if needed
	if req.Header.Get("X-Forwarded-Host") == "" {
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
	req.Header.Set("X-Forwarded-Proto", "http")
}

func (rp *ReverseProxy) errorHandler(w http.ResponseWriter, req *http.Request, err error) {
	rp.logger.Error("Reverse proxy error", "error", err, "url", req.URL.String())
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
