package client

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryTransport_Retries5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer srv.Close()

	rt := newRetryTransport(http.DefaultTransport)
	rt.baseDelay = 5 * time.Millisecond

	client := &http.Client{Transport: rt}
	req, _ := http.NewRequest("GET", srv.URL+"/v1/models", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode: got %d want 200", resp.StatusCode)
	}
	if calls.Load() != 3 {
		t.Errorf("calls: got %d want 3 (two 503s + one 200)", calls.Load())
	}
}

func TestRetryTransport_DoesNotRetryDeploymentCreate(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(503)
	}))
	defer srv.Close()

	rt := newRetryTransport(http.DefaultTransport)
	rt.baseDelay = 5 * time.Millisecond
	client := &http.Client{Transport: rt}

	req, _ := http.NewRequest("POST", srv.URL+"/v1/deployments", nil)
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 call (no retry on POST /v1/deployments), got %d", calls.Load())
	}
}

func TestRetryTransport_HonoursRetryAfter(t *testing.T) {
	var (
		calls    atomic.Int32
		firstAt  time.Time
		secondAt time.Time
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		switch n {
		case 1:
			firstAt = time.Now()
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(503)
		default:
			secondAt = time.Now()
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	rt := newRetryTransport(http.DefaultTransport)
	rt.baseDelay = 5 * time.Millisecond
	client := &http.Client{Transport: rt}

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	defer resp.Body.Close()
	gap := secondAt.Sub(firstAt)
	if gap < 950*time.Millisecond {
		t.Errorf("expected ~1s gap from Retry-After, got %v", gap)
	}
}
