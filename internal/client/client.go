// Package client is a typed HTTP wrapper around the Parel gateway.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.parel.cloud"
	DefaultTimeout = 60 * time.Second
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	UserAgent  string
	Verbose    bool
}

type Options struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
	Verbose bool
	Version string
}

func New(opts Options) *Client {
	base := opts.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	httpClient := &http.Client{
		Transport: newRetryTransport(http.DefaultTransport),
		Timeout:   timeout,
	}
	return &Client{
		BaseURL:    strings.TrimRight(base, "/"),
		APIKey:     opts.APIKey,
		HTTPClient: httpClient,
		UserAgent:  fmt.Sprintf("parel-cli/%s go/%s %s/%s", version, strings.TrimPrefix(runtime.Version(), "go"), runtime.GOOS, runtime.GOARCH),
		Verbose:    opts.Verbose,
	}
}

func (c *Client) WithAPIKey(key string) *Client {
	cp := *c
	cp.APIKey = key
	return &cp
}

func (c *Client) buildURL(path string, query url.Values) string {
	u := c.BaseURL + path
	if len(query) == 0 {
		return u
	}
	return u + "?" + query.Encode()
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(path, query), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	return req, nil
}

// do executes the request and decodes a JSON response into out (if non-nil).
// Returns a typed *ParelError on non-2xx responses.
func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return wrapNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(resp.Body)
		return parseHTTPError(resp.StatusCode, buf, resp.Header)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}

// doRaw returns the open response (caller must close body) for streaming.
func (c *Client) doRaw(req *http.Request) (*http.Response, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, wrapNetworkError(err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		buf, _ := io.ReadAll(resp.Body)
		return nil, parseHTTPError(resp.StatusCode, buf, resp.Header)
	}
	return resp, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) delete(ctx context.Context, path string, out any) error {
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}
