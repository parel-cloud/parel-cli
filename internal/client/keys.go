package client

import "context"

type CreateAPIKeyRequest struct {
	Name            string `json:"name"`
	Env             string `json:"env,omitempty"`
	PIIMode         string `json:"pii_mode,omitempty"`
	ProtectionLevel string `json:"protection_level,omitempty"`
}

type CreateAPIKeyResponse struct {
	ID              string `json:"id"`
	Key             string `json:"key"`
	KeyPrefix       string `json:"key_prefix"`
	Name            string `json:"name"`
	Env             string `json:"env,omitempty"`
	PIIMode         string `json:"pii_mode,omitempty"`
	ProtectionLevel string `json:"protection_level,omitempty"`
	CreatedAt       string `json:"created_at"`
}

func (c *Client) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	var out CreateAPIKeyResponse
	if err := c.post(ctx, "/v1/tenants/me/api-keys", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListAPIKeys(ctx context.Context) (*APIKeyListResponse, error) {
	return c.Whoami(ctx)
}

type RevokeAPIKeyResponse struct {
	Revoked bool   `json:"revoked"`
	ID      string `json:"id"`
}

func (c *Client) RevokeAPIKey(ctx context.Context, id string) (*RevokeAPIKeyResponse, error) {
	var out RevokeAPIKeyResponse
	if err := c.delete(ctx, "/v1/tenants/me/api-keys/"+id, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
