package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestProxy(t *testing.T, upstream string, upstreamByom string) *httptest.Server {
	t.Helper()
	p, err := New(Options{
		BaseURL:  upstream,
		APIKey:   "pk-secret",
		Upstream: upstreamByom,
	})
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(p.Handler())
}

func TestProxy_AuthorizationStripAndInject(t *testing.T) {
	var seenAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat_1"}`))
	}))
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL, "")
	defer proxy.Close()

	body := []byte(`{"model":"qwen3-max","messages":[{"role":"user","content":"hi"}]}`)
	req, _ := http.NewRequest("POST", proxy.URL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer client-supplied-leaky-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if seenAuth != "Bearer pk-secret" {
		t.Errorf("Authorization should be replaced with profile key, got %q", seenAuth)
	}
}

func TestProxy_RewritesModelWhenUpstreamSet(t *testing.T) {
	var seenBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL, "byom-abc-123")
	defer proxy.Close()

	body := []byte(`{"model":"qwen3-max","messages":[]}`)
	req, _ := http.NewRequest("POST", proxy.URL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var got map[string]any
	if err := json.Unmarshal(seenBody, &got); err != nil {
		t.Fatalf("upstream body not JSON: %v (raw=%q)", err, string(seenBody))
	}
	if got["model"] != "byom-abc-123" {
		t.Errorf("model field: got %v want byom-abc-123", got["model"])
	}
}

func TestProxy_PassesThroughWithoutModelRewrite(t *testing.T) {
	var seenBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL, "")
	defer proxy.Close()

	body := []byte(`{"model":"gpt-5.4","messages":[]}`)
	req, _ := http.NewRequest("POST", proxy.URL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if !bytes.Equal(body, seenBody) {
		t.Errorf("body should pass through unchanged.\nsent: %q\nseen: %q", string(body), string(seenBody))
	}
}

func TestProxy_HealthzReports(t *testing.T) {
	proxy := newTestProxy(t, "https://api.parel.cloud", "byom-xyz")
	defer proxy.Close()

	resp, err := http.Get(proxy.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz status: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	for _, want := range []string{`"ok":true`, `"upstream":"https://api.parel.cloud"`, `"upstream_byom":"byom-xyz"`} {
		if !strings.Contains(got, want) {
			t.Errorf("healthz body missing %q. got=%q", want, got)
		}
	}
}

func TestProxy_RejectsUnknownPaths(t *testing.T) {
	proxy := newTestProxy(t, "http://upstream.invalid", "")
	defer proxy.Close()

	resp, _ := http.Get(proxy.URL + "/v1/models")
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for /v1/models, got %d", resp.StatusCode)
	}
}

func TestProxy_StreamsSSEChunksWithFlushInterval(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: chunk-1\n\n"))
		flusher.Flush()
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("data: chunk-2\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL, "")
	defer proxy.Close()

	body := bytes.NewReader([]byte(`{"model":"x"}`))
	req, _ := http.NewRequest("POST", proxy.URL+"/v1/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	for _, want := range []string{"chunk-1", "chunk-2", "[DONE]"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("expected %q in streamed body, got %q", want, string(got))
		}
	}
}
