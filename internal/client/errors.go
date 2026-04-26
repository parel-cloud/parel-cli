package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// ParelError is the public error type returned by every Client method on
// non-2xx responses. It mirrors the OpenAI-compat envelope:
//
//	{ "error": { "message", "type", "code", "param", "request_id" } }
//
// Status-aware Is* helpers map gateway error codes to typed predicates.
type ParelError struct {
	StatusCode int
	Message    string
	Type       string
	Code       string
	Param      string
	RequestID  string
	RetryAfter int
	Raw        []byte
}

func (e *ParelError) Error() string {
	parts := []string{fmt.Sprintf("parel: %d", e.StatusCode)}
	if e.Code != "" {
		parts = append(parts, e.Code)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, " ")
}

// IsAuth maps to 401 / authentication_error / invalid_api_key.
func (e *ParelError) IsAuth() bool {
	return e.StatusCode == http.StatusUnauthorized || e.Code == "invalid_api_key" || e.Code == "authentication_error"
}

func (e *ParelError) IsPermission() bool {
	return e.StatusCode == http.StatusForbidden || e.Code == "permission_denied"
}

func (e *ParelError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound || e.Code == "not_found" || e.Code == "invalid_model"
}

func (e *ParelError) IsRateLimit() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.Code == "rate_limit_exceeded"
}

func (e *ParelError) IsBudgetExceeded() bool {
	return e.StatusCode == http.StatusPaymentRequired || e.Code == "budget_exceeded"
}

func (e *ParelError) IsDeploymentNotReady() bool {
	return e.Code == "deployment_not_ready"
}

func (e *ParelError) IsCapacityExhausted() bool {
	return e.Code == "capacity_exhausted" || e.Code == "all_providers_capacity_exhausted"
}

func (e *ParelError) IsProviderError() bool {
	return e.Code == "provider_error" || e.Code == "upstream_error"
}

func (e *ParelError) IsTimeout() bool {
	return e.StatusCode == http.StatusGatewayTimeout || e.Code == "timeout"
}

func (e *ParelError) IsValidation() bool {
	return e.StatusCode == http.StatusUnprocessableEntity || e.Code == "validation_error"
}

func (e *ParelError) IsConflict() bool {
	return e.StatusCode == http.StatusConflict || e.Code == "conflict"
}

func (e *ParelError) IsServerError() bool {
	return e.StatusCode >= 500
}

// envelope mirrors the gateway's ErrorResponse / ErrorDetail.
type envelope struct {
	Error struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      string `json:"code"`
		Param     string `json:"param"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func parseHTTPError(status int, body []byte, header http.Header) *ParelError {
	pe := &ParelError{StatusCode: status, Raw: body}

	var env envelope
	if len(body) > 0 && json.Unmarshal(body, &env) == nil && env.Error.Message != "" {
		pe.Message = env.Error.Message
		pe.Type = env.Error.Type
		pe.Code = env.Error.Code
		pe.Param = env.Error.Param
		pe.RequestID = env.Error.RequestID
	} else if len(body) > 0 {
		pe.Message = strings.TrimSpace(string(body))
	} else {
		pe.Message = http.StatusText(status)
	}

	if pe.RequestID == "" {
		pe.RequestID = header.Get("X-Request-Id")
	}
	if v := header.Get("Retry-After"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			pe.RetryAfter = n
		}
	}
	return pe
}

// wrapNetworkError converts low-level net errors into *ParelError so callers
// can switch on a single type.
func wrapNetworkError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return &ParelError{StatusCode: 0, Code: "canceled", Message: err.Error()}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &ParelError{StatusCode: http.StatusGatewayTimeout, Code: "timeout", Message: err.Error()}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &ParelError{StatusCode: http.StatusGatewayTimeout, Code: "timeout", Message: err.Error()}
	}
	return &ParelError{StatusCode: 0, Code: "network_error", Message: err.Error()}
}

// AsParelError extracts a *ParelError from any error chain.
func AsParelError(err error) (*ParelError, bool) {
	if err == nil {
		return nil, false
	}
	var pe *ParelError
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}
