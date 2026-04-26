package client

import (
	"context"
	"encoding/json"
	"net/url"
)

type Model struct {
	ID             string          `json:"id"`
	DisplayName    string          `json:"display_name,omitempty"`
	ModelType      string          `json:"model_type,omitempty"`
	Provider       string          `json:"provider,omitempty"`
	Source         string          `json:"source,omitempty"`
	Status         string          `json:"status,omitempty"`
	IsReady        *bool           `json:"is_ready,omitempty"`
	NotReadyReason string          `json:"not_ready_reason,omitempty"`
	Badges         []string        `json:"badges,omitempty"`
	Pricing        json.RawMessage `json:"pricing,omitempty"`
	PricingKind    string          `json:"pricing_kind,omitempty"`
	Capabilities   json.RawMessage `json:"capabilities,omitempty"`
	TierAccess     json.RawMessage `json:"tier_access,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

type ModelList struct {
	Data []Model `json:"data"`
}

func (c *Client) ListModels(ctx context.Context) (*ModelList, error) {
	var out ModelList
	if err := c.get(ctx, "/v1/models", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetModel(ctx context.Context, id string) (*Model, error) {
	var m Model
	q := url.Values{}
	if err := c.get(ctx, "/v1/models/"+url.PathEscape(id), q, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
