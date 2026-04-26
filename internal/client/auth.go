package client

import (
	"context"
	"time"
)

type APIKeyInfo struct {
	ID              string     `json:"id"`
	KeyPrefix       string     `json:"key_prefix"`
	Name            string     `json:"name"`
	Env             string     `json:"env,omitempty"`
	PIIMode         string     `json:"pii_mode,omitempty"`
	ProtectionLevel string     `json:"protection_level,omitempty"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
}

type APIKeyListResponse struct {
	Data  []APIKeyInfo `json:"data"`
	Total int          `json:"total"`
}

// Whoami probes the gateway with the current API key. A successful response
// implies the key is valid; the returned APIKeyListResponse lets callers
// discover the active key prefix without a separate /auth/me call.
func (c *Client) Whoami(ctx context.Context) (*APIKeyListResponse, error) {
	var out APIKeyListResponse
	if err := c.get(ctx, "/v1/tenants/me/api-keys", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
