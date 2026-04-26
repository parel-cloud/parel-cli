package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamSSE_ParsesChunksAndDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" there\"}}]}\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, APIKey: "test"})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/", nil, nil)
	events, errs := c.streamSSE(context.Background(), req)

	var collected []string
	doneSeen := false
	for ev := range events {
		if ev.IsDone() {
			doneSeen = true
			continue
		}
		collected = append(collected, string(ev.Data))
	}
	if err := <-errs; err != nil {
		t.Fatalf("unexpected stream err: %v", err)
	}
	if !doneSeen {
		t.Error("expected [DONE] sentinel")
	}
	if len(collected) != 2 {
		t.Errorf("expected 2 data frames, got %d: %v", len(collected), collected)
	}
}

func TestStreamSSE_RejectsNonSSEContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, APIKey: "test"})
	req, _ := c.newRequest(context.Background(), http.MethodGet, "/", nil, nil)
	_, errs := c.streamSSE(context.Background(), req)
	if err := <-errs; err == nil {
		t.Fatal("expected an error on non-SSE content type")
	}
}
