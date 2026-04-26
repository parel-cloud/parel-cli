package client

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

// retryTransport implements http.RoundTripper with bounded exponential backoff
// on transient gateway failures (502/503/504, network errors). It does NOT
// retry POSTs to /v1/deployments because deployment creation is not idempotent.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	baseDelay  time.Duration
}

func newRetryTransport(base http.RoundTripper) *retryTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &retryTransport{base: base, maxRetries: 3, baseDelay: 250 * time.Millisecond}
}

func (rt *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !rt.eligible(req) {
		return rt.base.RoundTrip(req)
	}

	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	var lastResp *http.Response
	var lastErr error
	for attempt := 0; attempt <= rt.maxRetries; attempt++ {
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := rt.base.RoundTrip(req)
		lastResp, lastErr = resp, err
		if err == nil && !shouldRetryStatus(resp.StatusCode) {
			return resp, nil
		}
		if attempt == rt.maxRetries {
			break
		}

		// Drain & close before retry
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		delay := backoff(rt.baseDelay, attempt)
		if resp != nil && resp.Header.Get("Retry-After") != "" {
			if n, perr := strconv.Atoi(resp.Header.Get("Retry-After")); perr == nil && n > 0 {
				delay = time.Duration(n) * time.Second
			}
		}

		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}

		if err != nil && !isRetryableNetworkError(err) {
			break
		}
	}
	return lastResp, lastErr
}

func (rt *retryTransport) eligible(req *http.Request) bool {
	if req.Method == http.MethodPost && req.URL.Path == "/v1/deployments" {
		return false
	}
	if req.Method == http.MethodPost && req.URL.Path == "/v1/tenants/me/api-keys" {
		// Key creation also non-idempotent.
		return false
	}
	return true
}

func shouldRetryStatus(status int) bool {
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var ne interface{ Temporary() bool }
	if errors.As(err, &ne) && ne.Temporary() {
		return true
	}
	var to interface{ Timeout() bool }
	if errors.As(err, &to) && to.Timeout() {
		return true
	}
	return false
}

func backoff(base time.Duration, attempt int) time.Duration {
	d := base
	for i := 0; i < attempt; i++ {
		d *= 2
	}
	if d > 4*time.Second {
		d = 4 * time.Second
	}
	return d
}
