package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// Speak posts a TTS request and returns the raw audio bytes.
func (c *Client) Speak(ctx context.Context, req SpeechRequest) ([]byte, string, error) {
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/v1/audio/speech", nil, req)
	if err != nil {
		return nil, "", err
	}
	httpReq.Header.Set("Accept", "audio/*")
	resp, err := c.doRaw(httpReq)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

type TranscribeOptions struct {
	Model          string
	Language       string
	ResponseFormat string
	Prompt         string
	Temperature    *float64
}

type TranscribeResponse struct {
	Text     string          `json:"text"`
	Language string          `json:"language,omitempty"`
	Duration float64         `json:"duration,omitempty"`
	Raw      json.RawMessage `json:"-"`
}

// Transcribe uploads a file via multipart/form-data and decodes the JSON
// response. responseFormat="text" returns the raw text in TranscribeResponse.Text.
func (c *Client) Transcribe(ctx context.Context, filePath string, opts TranscribeOptions) (*TranscribeResponse, error) {
	if opts.Model == "" {
		return nil, fmt.Errorf("transcribe: model is required")
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("model", opts.Model); err != nil {
		return nil, err
	}
	if opts.Language != "" {
		_ = w.WriteField("language", opts.Language)
	}
	if opts.ResponseFormat != "" {
		_ = w.WriteField("response_format", opts.ResponseFormat)
	}
	if opts.Prompt != "" {
		_ = w.WriteField("prompt", opts.Prompt)
	}
	if opts.Temperature != nil {
		_ = w.WriteField("temperature", fmt.Sprintf("%v", *opts.Temperature))
	}

	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.buildURL("/v1/audio/transcriptions", nil), &body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", c.UserAgent)
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.doRaw(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	out := &TranscribeResponse{Raw: raw}
	if err := json.Unmarshal(raw, out); err != nil {
		// Some response_format=text returns plain text
		out.Text = string(raw)
	}
	return out, nil
}

type Voice struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Language string `json:"language,omitempty"`
	Provider string `json:"provider,omitempty"`
	Gender   string `json:"gender,omitempty"`
}

type VoiceList struct {
	Data []Voice `json:"data"`
}

func (c *Client) ListVoices(ctx context.Context) (*VoiceList, error) {
	var out VoiceList
	if err := c.get(ctx, "/v1/audio/voices", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
