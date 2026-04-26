package client

import (
	"context"
	"encoding/json"
	"net/url"
)

// Deployment is a partial typed view of the gateway response. The Raw field
// preserves the full JSON for callers that need additional fields (admin tooling).
type Deployment struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	HuggingfaceID      string  `json:"huggingface_id"`
	DisplayName        string  `json:"display_name"`
	GPUTier            string  `json:"gpu_tier"`
	GPULabel           string  `json:"gpu_label"`
	Provider           string  `json:"provider"`
	ParelModelID       string  `json:"parel_model_id"`
	Quantization       string  `json:"quantization"`
	Status             string  `json:"status"`
	IdleTimeoutMinutes int     `json:"idle_timeout_minutes"`
	BudgetLimitUSD     float64 `json:"budget_limit_usd"`
	BudgetSpentUSD     float64 `json:"budget_spent_usd"`
	ParelPricePerHr    float64 `json:"parel_price_per_hr"`
	HourlyCost         float64 `json:"hourly_cost"`
	InvokeBaseURL      string  `json:"invoke_base_url"`
	CreatedAt          string  `json:"created_at"`
	StartedAt          string  `json:"started_at,omitempty"`
	StoppedAt          string  `json:"stopped_at,omitempty"`
	LastRequestAt      string  `json:"last_request_at,omitempty"`
	ErrorMessage       string  `json:"error_message,omitempty"`
	ETAAvgMinutes      float64 `json:"eta_avg_minutes,omitempty"`
	ETASampleCount     int     `json:"eta_sample_count,omitempty"`
	ETABasis           string  `json:"eta_basis,omitempty"`
}

type CreateDeploymentRequest struct {
	HuggingfaceID         string   `json:"huggingface_id"`
	Name                  string   `json:"name"`
	GPUTier               string   `json:"gpu_tier,omitempty"`
	Provider              string   `json:"provider,omitempty"`
	ProviderMode          string   `json:"provider_mode,omitempty"`
	AllocationMode        string   `json:"allocation_mode,omitempty"`
	PreferredProviders    []string `json:"preferred_providers,omitempty"`
	Quantization          string   `json:"quantization,omitempty"`
	IdleTimeoutMinutes    int      `json:"idle_timeout_minutes,omitempty"`
	BudgetLimitUSD        float64  `json:"budget_limit_usd,omitempty"`
	HFToken               string   `json:"hf_token,omitempty"`
	MaxModelLen           int      `json:"max_model_len,omitempty"`
	ExpectedMaxPricePerHr float64  `json:"expected_max_price_per_hr,omitempty"`
}

type DeploymentList struct {
	Data []Deployment `json:"data"`
}

func (c *Client) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (*Deployment, error) {
	var dep Deployment
	if err := c.post(ctx, "/v1/deployments", req, &dep); err != nil {
		return nil, err
	}
	return &dep, nil
}

func (c *Client) ListDeployments(ctx context.Context, status string) (*DeploymentList, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	var out DeploymentList
	if err := c.get(ctx, "/v1/deployments", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	var dep Deployment
	if err := c.get(ctx, "/v1/deployments/"+url.PathEscape(id), nil, &dep); err != nil {
		return nil, err
	}
	return &dep, nil
}

func (c *Client) StartDeployment(ctx context.Context, id string) (*Deployment, error) {
	var dep Deployment
	if err := c.post(ctx, "/v1/deployments/"+url.PathEscape(id)+"/start", nil, &dep); err != nil {
		return nil, err
	}
	return &dep, nil
}

func (c *Client) StopDeployment(ctx context.Context, id string) (*Deployment, error) {
	var dep Deployment
	if err := c.post(ctx, "/v1/deployments/"+url.PathEscape(id)+"/stop", nil, &dep); err != nil {
		return nil, err
	}
	return &dep, nil
}

type DeleteDeploymentResponse struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

func (c *Client) DeleteDeployment(ctx context.Context, id string) (*DeleteDeploymentResponse, error) {
	var out DeleteDeploymentResponse
	if err := c.delete(ctx, "/v1/deployments/"+url.PathEscape(id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type DeploymentEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Status    string          `json:"status,omitempty"`
	Message   string          `json:"message,omitempty"`
	CreatedAt string          `json:"created_at"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

type DeploymentEventList struct {
	Data []DeploymentEvent `json:"data"`
}

func (c *Client) ListDeploymentEvents(ctx context.Context, id string, since string) (*DeploymentEventList, error) {
	q := url.Values{}
	if since != "" {
		q.Set("since", since)
	}
	var out DeploymentEventList
	if err := c.get(ctx, "/v1/deployments/"+url.PathEscape(id)+"/events", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeploymentMetrics(ctx context.Context, id string) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.get(ctx, "/v1/deployments/"+url.PathEscape(id)+"/metrics", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeploymentBilling(ctx context.Context, id string) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.get(ctx, "/v1/deployments/"+url.PathEscape(id)+"/billing", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// HFValidateRequest matches POST /v1/hf/validate.
type HFValidateRequest struct {
	ModelID string `json:"model_id"`
	HFToken string `json:"hf_token,omitempty"`
}

type HFValidateResponse struct {
	Valid               bool            `json:"valid"`
	ModelID             string          `json:"model_id"`
	Architecture        string          `json:"architecture,omitempty"`
	ModelType           string          `json:"model_type,omitempty"`
	VRAMFp16GB          float64         `json:"vram_fp16_gb,omitempty"`
	VRAMInt4GB          float64         `json:"vram_int4_gb,omitempty"`
	RecommendedGPUTier  string          `json:"recommended_gpu_tier,omitempty"`
	GhostTestPassed     bool            `json:"ghost_test_passed"`
	GhostTestSignature  string          `json:"ghost_test_signature,omitempty"`
	NeedsNewVLLM        bool            `json:"needs_new_vllm,omitempty"`
	ProviderCompatHint  string          `json:"provider_compat_hint,omitempty"`
	Errors              []string        `json:"errors,omitempty"`
	Raw                 json.RawMessage `json:"-"`
}

func (c *Client) ValidateHF(ctx context.Context, req HFValidateRequest) (*HFValidateResponse, error) {
	var out HFValidateResponse
	if err := c.post(ctx, "/v1/hf/validate", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type DeploymentPreview struct {
	HuggingfaceID  string  `json:"huggingface_id"`
	GPUTier        string  `json:"gpu_tier"`
	Provider       string  `json:"provider"`
	ETAMinutes     float64 `json:"eta_minutes"`
	HourlyCostUSD  float64 `json:"hourly_cost_usd"`
	IsCached       bool    `json:"is_cached"`
	CacheStatus    string  `json:"cache_status,omitempty"`
	Notes          string  `json:"notes,omitempty"`
}

func (c *Client) PreviewDeployment(ctx context.Context, hfID, gpuTier string) (*DeploymentPreview, error) {
	q := url.Values{}
	q.Set("huggingface_id", hfID)
	if gpuTier != "" {
		q.Set("gpu_tier", gpuTier)
	}
	var out DeploymentPreview
	if err := c.get(ctx, "/v1/deployments/preview", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type GPUTier struct {
	ID            string  `json:"id"`
	DisplayName   string  `json:"display_name"`
	VRAMGB        int     `json:"vram_gb"`
	PricePerHrUSD float64 `json:"price_per_hr_usd"`
	Provider      string  `json:"provider,omitempty"`
	Capacity      string  `json:"capacity,omitempty"`
}

type GPUTierList struct {
	Data []GPUTier `json:"data"`
}

func (c *Client) ListGPUTiers(ctx context.Context, live bool) (*GPUTierList, error) {
	path := "/v1/gpu-tiers"
	if live {
		path = "/v1/gpu-tiers/live"
	}
	var out GPUTierList
	if err := c.get(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type DeploymentTemplate struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	HuggingfaceID string          `json:"huggingface_id"`
	GPUTier       string          `json:"gpu_tier"`
	Description   string          `json:"description,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
	Defaults      json.RawMessage `json:"defaults,omitempty"`
}

type DeploymentTemplateList struct {
	Data []DeploymentTemplate `json:"data"`
}

func (c *Client) ListDeploymentTemplates(ctx context.Context) (*DeploymentTemplateList, error) {
	var out DeploymentTemplateList
	if err := c.get(ctx, "/v1/deployment-templates", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
