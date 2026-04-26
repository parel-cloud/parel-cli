package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_DefaultBaseURLAndUA(t *testing.T) {
	c := New(Options{APIKey: "pk-test", Version: "0.1.0"})
	if c.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL: got %q want %q", c.BaseURL, DefaultBaseURL)
	}
	if !strings.HasPrefix(c.UserAgent, "parel-cli/0.1.0") {
		t.Errorf("UserAgent: got %q", c.UserAgent)
	}
}

func TestClient_AuthorizationHeader(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, APIKey: "pk-secret"})
	if _, err := c.Whoami(context.Background()); err != nil {
		t.Fatalf("Whoami err: %v", err)
	}
	if seenAuth != "Bearer pk-secret" {
		t.Errorf("Authorization: got %q want 'Bearer pk-secret'", seenAuth)
	}
}

func TestClient_ParsesParelErrorOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key","type":"authentication_error","code":"invalid_api_key"}}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, APIKey: "pk-bad"})
	_, err := c.Whoami(context.Background())
	pe, ok := AsParelError(err)
	if !ok {
		t.Fatalf("expected *ParelError, got %T: %v", err, err)
	}
	if !pe.IsAuth() {
		t.Errorf("expected IsAuth() == true, got code=%q", pe.Code)
	}
}
