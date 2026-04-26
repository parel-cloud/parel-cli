package client

import (
	"net/http"
	"testing"
)

func TestParseHTTPError_OpenAIEnvelope(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid API key","type":"authentication_error","code":"invalid_api_key","request_id":"req_abc"}}`)
	headers := http.Header{}
	headers.Set("X-Request-Id", "req_xyz")

	pe := parseHTTPError(401, body, headers)

	if pe == nil {
		t.Fatal("expected non-nil ParelError")
	}
	if pe.StatusCode != 401 {
		t.Errorf("StatusCode: got %d want 401", pe.StatusCode)
	}
	if pe.Code != "invalid_api_key" {
		t.Errorf("Code: got %q want invalid_api_key", pe.Code)
	}
	if !pe.IsAuth() {
		t.Error("expected IsAuth() == true")
	}
	if pe.RequestID != "req_abc" {
		t.Errorf("RequestID: prefer envelope value, got %q", pe.RequestID)
	}
}

func TestParseHTTPError_RetryAfterHeader(t *testing.T) {
	body := []byte(`{"error":{"message":"slow down","type":"rate_limit_error","code":"rate_limit_exceeded"}}`)
	headers := http.Header{}
	headers.Set("Retry-After", "42")

	pe := parseHTTPError(429, body, headers)

	if pe.RetryAfter != 42 {
		t.Errorf("RetryAfter: got %d want 42", pe.RetryAfter)
	}
	if !pe.IsRateLimit() {
		t.Error("expected IsRateLimit() == true")
	}
}

func TestParseHTTPError_NonJSONBody(t *testing.T) {
	pe := parseHTTPError(503, []byte("upstream temporarily unavailable"), http.Header{})

	if pe.Message != "upstream temporarily unavailable" {
		t.Errorf("Message: got %q", pe.Message)
	}
	if !pe.IsServerError() {
		t.Error("expected IsServerError() == true")
	}
}

func TestParseHTTPError_EmptyBody(t *testing.T) {
	pe := parseHTTPError(404, nil, http.Header{})
	if pe.Message != "Not Found" {
		t.Errorf("Message: expected stdlib status text, got %q", pe.Message)
	}
	if !pe.IsNotFound() {
		t.Error("expected IsNotFound() == true")
	}
}

func TestAsParelError(t *testing.T) {
	pe := &ParelError{StatusCode: 401, Code: "invalid_api_key"}
	if got, ok := AsParelError(pe); !ok || got != pe {
		t.Error("AsParelError should unwrap *ParelError directly")
	}
	if _, ok := AsParelError(nil); ok {
		t.Error("AsParelError(nil) should return false")
	}
}
