package client

import (
	"context"
	"encoding/json"
	"net/url"
)

func (c *Client) UsageSummary(ctx context.Context, from, to string) (json.RawMessage, error) {
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	var out json.RawMessage
	if err := c.get(ctx, "/v1/usage/summary", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UsageBudget(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.get(ctx, "/v1/usage/budget", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UsageSpending(ctx context.Context, by string) (json.RawMessage, error) {
	q := url.Values{}
	if by != "" {
		q.Set("by", by)
	}
	var out json.RawMessage
	if err := c.get(ctx, "/v1/usage/spending", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UsageGPUBilling(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.get(ctx, "/v1/usage/gpu-billing", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
