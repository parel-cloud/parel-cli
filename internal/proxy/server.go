// Package proxy is the local OpenAI- and Anthropic-compatible HTTP server
// behind `parel proxy`. It is the killer feature for v0.1: any client that
// expects an OpenAI- or Anthropic-style endpoint can target localhost and the
// proxy forwards everything to the Parel gateway with the configured API key
// already attached.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const Version = "0.1.0"

// Routes that the proxy forwards 1:1 to the gateway. Any other path returns
// 404 — the proxy is intentionally narrow so users notice if a client is
// hitting a path it shouldn't.
var forwardedRoutes = map[string]struct{}{
	"/v1/chat/completions":               {},
	"/anthropic/v1/messages":             {},
	"/anthropic/v1/messages/count_tokens": {},
}

type Options struct {
	BaseURL  string // upstream gateway, e.g. https://api.parel.cloud
	APIKey   string // Bearer token applied to every forwarded request
	Bind     string // listen address (default 127.0.0.1)
	Port     int    // listen port (default 7878)
	Upstream string // optional BYOM deployment id ("byom-..."); when non-empty
	// the request body's `model` field is rewritten before forwarding
}

type Server struct {
	opts     Options
	upstream *url.URL
	rev      *httputil.ReverseProxy
	logger   func(format string, args ...any)
}

// New builds a configured proxy.Server. The HTTP server is created lazily by
// Start so callers can swap in a custom logger first.
func New(opts Options) (*Server, error) {
	if opts.BaseURL == "" {
		return nil, fmt.Errorf("proxy: BaseURL is required")
	}
	if opts.APIKey == "" {
		return nil, fmt.Errorf("proxy: APIKey is required")
	}
	if opts.Bind == "" {
		opts.Bind = "127.0.0.1"
	}
	if opts.Port == 0 {
		opts.Port = 7878
	}

	u, err := url.Parse(strings.TrimRight(opts.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("proxy: invalid BaseURL: %w", err)
	}

	s := &Server{opts: opts, upstream: u, logger: func(string, ...any) {}}
	s.rev = &httputil.ReverseProxy{
		Director:      s.director,
		FlushInterval: 100 * time.Millisecond, // critical for SSE streaming
		ErrorHandler:  s.errorHandler,
	}
	return s, nil
}

// SetLogger lets the CLI surface request lines.
func (s *Server) SetLogger(fn func(string, ...any)) {
	if fn != nil {
		s.logger = fn
	}
}

// Address returns the bound listen address, useful for printing the URL.
func (s *Server) Address() string {
	return fmt.Sprintf("%s:%d", s.opts.Bind, s.opts.Port)
}

// Handler exposes the http.Handler so callers can mount it under a custom
// listener (tests use httptest.NewServer(srv.Handler())).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/", s.dispatch)
	return mux
}

// Start runs the proxy until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.Address(),
		Handler: s.Handler(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":       true,
		"version":  Version,
		"upstream": s.opts.BaseURL,
		"upstream_byom": s.opts.Upstream,
	})
}

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	if _, ok := forwardedRoutes[r.URL.Path]; !ok {
		http.NotFound(w, r)
		return
	}
	s.logger("→ %s %s", r.Method, r.URL.Path)

	if s.opts.Upstream != "" && r.Body != nil && r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			http.Error(w, "proxy: read body: "+err.Error(), http.StatusBadGateway)
			return
		}
		if rewritten, err := rewriteModel(body, s.opts.Upstream); err == nil {
			body = rewritten
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		r.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}
	s.rev.ServeHTTP(w, r)
}

func (s *Server) director(req *http.Request) {
	target := s.upstream
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
	req.Host = target.Host

	// Strip whatever the local client sent and inject our key. This is the
	// whole point of the proxy: secrets stay in the user's parel profile,
	// never in IDE config files.
	req.Header.Del("Authorization")
	req.Header.Del("X-Api-Key")
	req.Header.Set("Authorization", "Bearer "+s.opts.APIKey)
	req.Header.Set("User-Agent", "parel-cli-proxy/"+Version+" "+req.Header.Get("User-Agent"))
}

func (s *Server) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	s.logger("✗ %s %s: %v", r.Method, r.URL.Path, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": "parel proxy: upstream error: " + err.Error(),
			"type":    "proxy_error",
		},
	})
}

func rewriteModel(body []byte, target string) ([]byte, error) {
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		return body, err
	}
	generic["model"] = target
	return json.Marshal(generic)
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
