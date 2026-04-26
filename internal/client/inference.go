package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ChatMessage is the OpenAI-compat message envelope.
type ChatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Name    string          `json:"name,omitempty"`
}

func TextMessage(role, text string) ChatMessage {
	b, _ := json.Marshal(text)
	return ChatMessage{Role: role, Content: b}
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	User        string        `json:"user,omitempty"`
}

type ChatChoice struct {
	Index        int             `json:"index"`
	Message      ChatMessage     `json:"message,omitempty"`
	Delta        json.RawMessage `json:"delta,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
}

type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	var out ChatResponse
	if err := c.post(ctx, "/v1/chat/completions", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ChatStream returns a channel of SSEEvents whose Data is one OpenAI streaming
// chunk (or the [DONE] sentinel — caller must check IsDone()).
func (c *Client) ChatStream(ctx context.Context, req ChatRequest) (<-chan SSEEvent, <-chan error) {
	req.Stream = true
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/v1/chat/completions", nil, req)
	if err != nil {
		errs := make(chan error, 1)
		errs <- err
		close(errs)
		empty := make(chan SSEEvent)
		close(empty)
		return empty, errs
	}
	return c.streamSSE(ctx, httpReq)
}

// AnthropicMessages proxies to /anthropic/v1/messages. The Body is opaque so
// callers can pass the exact Anthropic SDK payload.
func (c *Client) AnthropicMessages(ctx context.Context, body json.RawMessage) (json.RawMessage, error) {
	var out json.RawMessage
	req, err := c.newRequest(ctx, http.MethodPost, "/anthropic/v1/messages", nil, json.RawMessage(body))
	if err != nil {
		return nil, err
	}
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) AnthropicCountTokens(ctx context.Context, body json.RawMessage) (json.RawMessage, error) {
	var out json.RawMessage
	req, err := c.newRequest(ctx, http.MethodPost, "/anthropic/v1/messages/count_tokens", nil, json.RawMessage(body))
	if err != nil {
		return nil, err
	}
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
	User  string `json:"user,omitempty"`
}

type EmbeddingObject struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type EmbeddingResponse struct {
	Object string            `json:"object"`
	Model  string            `json:"model"`
	Data   []EmbeddingObject `json:"data"`
	Usage  ChatUsage         `json:"usage"`
}

func (c *Client) Embeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("embeddings: model is required")
	}
	var out EmbeddingResponse
	if err := c.post(ctx, "/v1/embeddings", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
