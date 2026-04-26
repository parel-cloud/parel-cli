package client

import (
	"context"
	"encoding/json"
)

type VideoGenerateRequest struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Duration int    `json:"duration,omitempty"`
	Size     string `json:"size,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	User     string `json:"user,omitempty"`
}

type VideoGenerateResponse struct {
	TaskID string          `json:"task_id"`
	Status string          `json:"status"`
	Model  string          `json:"model,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

func (c *Client) GenerateVideo(ctx context.Context, req VideoGenerateRequest) (*VideoGenerateResponse, error) {
	var out VideoGenerateResponse
	if err := c.post(ctx, "/v1/videos/generations", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
