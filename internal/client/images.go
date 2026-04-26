package client

import (
	"context"
	"encoding/json"
)

type ImageGenerateRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	User           string `json:"user,omitempty"`
}

type ImageObject struct {
	URL          string `json:"url,omitempty"`
	B64JSON      string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageGenerateResponse covers both the synchronous (data-filled) and
// async (task_id-filled) shapes the gateway returns.
type ImageGenerateResponse struct {
	Created int64           `json:"created,omitempty"`
	Data    []ImageObject   `json:"data,omitempty"`
	TaskID  string          `json:"task_id,omitempty"`
	Status  string          `json:"status,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

func (c *Client) GenerateImage(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResponse, error) {
	var out ImageGenerateResponse
	if err := c.post(ctx, "/v1/images/generations", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
