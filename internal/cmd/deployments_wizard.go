package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/parel-cloud/parel-cli/internal/client"
)

// wizardCreateDeployment walks the user through every CreateDeploymentRequest
// field. CLI flags act as presets that pre-fill prompts; --yes skips prompts
// entirely and uses (preset || default). The gateway is consulted twice:
//
//  1. POST /v1/hf/validate to get architecture + recommended GPU + warnings
//  2. GET /v1/deployments/preview to get ETA + hourly cost + cache state
//
// Returns (req, false, nil) when the user declines the final confirmation —
// callers should print "Aborted" and return cleanly.
func wizardCreateDeployment(parent context.Context, c *client.Client) (client.CreateDeploymentRequest, bool, error) {
	var zero client.CreateDeploymentRequest

	hfID, err := wizardAskHFID(depCreateHFID, depCreateYes)
	if err != nil {
		return zero, false, err
	}
	if hfID == "" {
		return zero, false, errors.New("hf-id is required")
	}

	// Validate against the gateway. We always do this — even with --yes — so
	// the price-confirmation gate has accurate VRAM data.
	val, err := wizardValidate(parent, c, hfID)
	if err != nil {
		return zero, false, err
	}
	if !val.Valid {
		printValidateFailure(val)
		if !depCreateYes {
			return zero, false, errors.New("HF validation failed")
		}
	}

	gpu, err := wizardPickGPU(parent, c, val)
	if err != nil {
		return zero, false, err
	}

	quant, err := wizardPickQuantization(depCreateQuant, depCreateYes)
	if err != nil {
		return zero, false, err
	}

	providerMode, providerName, err := wizardPickProvider(depCreateProvider, depCreateProviderMode, depCreateAdvanced, depCreateYes)
	if err != nil {
		return zero, false, err
	}

	idle, err := wizardAskInt("Idle timeout (dk)",
		"Bu süre boyunca istek gelmezse pod uyur; tekrar istek gelince 30-60sn'de uyanır.",
		depCreateIdleTimeout, 15, depCreateYes)
	if err != nil {
		return zero, false, err
	}

	// Budget default = backend'in preview önerisi (yoksa fallback).
	prev := wizardPreview(parent, c, hfID, gpu)
	budgetDefault := suggestBudget(prev, depCreateBudget)
	budget, err := wizardAskFloat("Budget cap (USD)",
		"Bu deployment için üst harcama eşiği. Aşılınca otomatik durur.",
		depCreateBudget, budgetDefault, depCreateYes)
	if err != nil {
		return zero, false, err
	}

	req := client.CreateDeploymentRequest{
		HuggingfaceID:         hfID,
		Name:                  fallback(depCreateName, defaultDeploymentName(hfID)),
		GPUTier:               gpu,
		Provider:              providerName,
		ProviderMode:          providerMode,
		AllocationMode:        depCreateAllocation,
		Quantization:          normalizeQuantization(quant),
		IdleTimeoutMinutes:    idle,
		BudgetLimitUSD:        budget,
		MaxModelLen:           depCreateMaxModelLen,
		ExpectedMaxPricePerHr: depCreateMaxPrice,
		HFToken:               depCreateHFToken,
	}

	printSummary(req, prev)

	if depCreateYes {
		return req, true, nil
	}
	confirm := true
	if err := survey.AskOne(&survey.Confirm{Message: "Onaylıyor musun?", Default: true}, &confirm); err != nil {
		return zero, false, err
	}
	return req, confirm, nil
}

// ---------------------------------------------------------------------------
// HF id
// ---------------------------------------------------------------------------

var suggestedHFIDs = []string{
	"Qwen/Qwen2.5-7B-Instruct",
	"Qwen/Qwen2.5-Coder-32B-Instruct",
	"meta-llama/Llama-3.3-70B-Instruct",
	"mistralai/Mistral-Small-3.1-24B-Instruct",
	"deepseek-ai/DeepSeek-Coder-V2-Lite-Instruct",
}

func wizardAskHFID(preset string, nonInteractive bool) (string, error) {
	if preset != "" {
		return strings.TrimSpace(preset), nil
	}
	if nonInteractive {
		return "", nil
	}
	var picked string
	prompt := &survey.Input{
		Message: "HuggingFace model id:",
		Help: "Tab ile öneri listesinden seç, ya da kendi HF id'ni yaz. " +
			"Örnek: Qwen/Qwen2.5-7B-Instruct",
		Suggest: func(toComplete string) []string {
			out := make([]string, 0, len(suggestedHFIDs))
			needle := strings.ToLower(toComplete)
			for _, s := range suggestedHFIDs {
				if needle == "" || strings.Contains(strings.ToLower(s), needle) {
					out = append(out, s)
				}
			}
			if len(out) == 0 {
				return suggestedHFIDs
			}
			return out
		},
	}
	if err := survey.AskOne(prompt, &picked); err != nil {
		return "", err
	}
	picked = strings.TrimSpace(picked)
	if picked == "" {
		// Boş bırakıldıysa autocomplete'in ilki (karar #1)
		picked = suggestedHFIDs[0]
		fmt.Fprintf(os.Stderr, "(boş — varsayılan: %s)\n", picked)
	}
	return picked, nil
}

// ---------------------------------------------------------------------------
// HF validate
// ---------------------------------------------------------------------------

func wizardValidate(parent context.Context, c *client.Client, hfID string) (*client.HFValidateResponse, error) {
	fmt.Fprintln(os.Stderr, "  HF validator çalışıyor...")
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	val, err := c.ValidateHF(ctx, client.HFValidateRequest{ModelID: hfID, HFToken: depCreateHFToken})
	if err != nil {
		return nil, printError(err)
	}
	printValidateSuccess(val)
	return val, nil
}

func printValidateSuccess(v *client.HFValidateResponse) {
	if !v.Valid {
		return
	}
	fmt.Fprintf(os.Stderr, "  ✓ %s\n", v.ModelID)
	if v.Architecture != "" {
		fmt.Fprintf(os.Stderr, "    Mimari       %s\n", v.Architecture)
	}
	if v.VRAMFp16GB > 0 {
		fmt.Fprintf(os.Stderr, "    VRAM fp16    %.1f GB\n", v.VRAMFp16GB)
	}
	if v.VRAMInt4GB > 0 {
		fmt.Fprintf(os.Stderr, "    VRAM int4    %.1f GB\n", v.VRAMInt4GB)
	}
	if v.RecommendedGPUTier != "" {
		fmt.Fprintf(os.Stderr, "    Önerilen     %s\n", v.RecommendedGPUTier)
	}
	if v.NeedsNewVLLM {
		fmt.Fprintln(os.Stderr, "    ⚠ Bu model daha yeni vLLM gerektirebilir; provider sınırlı.")
	}
}

func printValidateFailure(v *client.HFValidateResponse) {
	fmt.Fprintf(os.Stderr, "  ✗ %s — doğrulama başarısız\n", v.ModelID)
	for _, e := range v.Errors {
		fmt.Fprintf(os.Stderr, "    - %s\n", e)
	}
	if v.GhostTestSignature != "" {
		fmt.Fprintf(os.Stderr, "    Ghost signature: %s\n", v.GhostTestSignature)
	}
	if v.ProviderCompatHint != "" {
		fmt.Fprintf(os.Stderr, "    Hint: %s\n", v.ProviderCompatHint)
	}
}

// ---------------------------------------------------------------------------
// GPU tier picker
// ---------------------------------------------------------------------------

func wizardPickGPU(parent context.Context, c *client.Client, val *client.HFValidateResponse) (string, error) {
	if depCreateGPUTier != "" {
		return depCreateGPUTier, nil
	}
	if depCreateYes && val != nil && val.RecommendedGPUTier != "" {
		return val.RecommendedGPUTier, nil
	}
	if depCreateYes {
		return "", errors.New("--yes ile çalıştırırken --gpu vermelisin (validator önermedi)")
	}

	tiers := wizardListTiers(parent, c)
	if len(tiers) == 0 {
		// Fallback: serbest input
		var custom string
		err := survey.AskOne(&survey.Input{Message: "GPU tier id:", Default: defaultIfEmpty(val.RecommendedGPUTier, "rtx-4090")}, &custom)
		return custom, err
	}

	options := make([]string, 0, len(tiers))
	defaultOpt := ""
	recVRAM := val.VRAMFp16GB
	for _, t := range tiers {
		marker := ""
		if val != nil && t.ID == val.RecommendedGPUTier {
			marker = "  [önerilen]"
		}
		warn := ""
		if recVRAM > 0 && float64(t.VRAMGB) < recVRAM*1.15 {
			warn = "  ⚠ VRAM dar"
		}
		opt := fmt.Sprintf("%-14s %3d GB · $%.4f/hr · %s%s%s",
			t.ID, t.VRAMGB, t.PricePerHrUSD, fallback(t.Capacity, "kapasite ?"), marker, warn)
		options = append(options, opt)
		if val != nil && t.ID == val.RecommendedGPUTier {
			defaultOpt = opt
		}
	}
	if defaultOpt == "" {
		defaultOpt = options[0]
	}

	var picked string
	if err := survey.AskOne(&survey.Select{
		Message: "GPU tier:",
		Options: options,
		Default: defaultOpt,
	}, &picked); err != nil {
		return "", err
	}
	idx := indexOf(options, picked)
	if idx < 0 {
		return "", fmt.Errorf("seçim eşleşmedi: %q", picked)
	}
	return tiers[idx].ID, nil
}

func wizardListTiers(parent context.Context, c *client.Client) []client.GPUTier {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	out, err := c.ListGPUTiers(ctx, true)
	if err != nil {
		// Live çağrı fail ettiyse cache'li listeyi dene.
		ctx2, cancel2 := context.WithTimeout(parent, 10*time.Second)
		defer cancel2()
		out2, err2 := c.ListGPUTiers(ctx2, false)
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ GPU tier listesi alınamadı, manuel yazman gerekecek (%v)\n", err)
			return nil
		}
		return out2.Data
	}
	return out.Data
}

// ---------------------------------------------------------------------------
// Quantization
// ---------------------------------------------------------------------------

var quantizationOptions = []struct {
	id   string
	desc string
}{
	{"auto", "auto       (HF validator karar versin — önerilen)"},
	{"fp16", "fp16       tam VRAM, vanilya"},
	{"fp8", "fp8        ~yarı VRAM, vLLM 0.10+"},
	{"awq", "awq        ~%30 VRAM, int4 GEMM"},
	{"gptq", "gptq       ~%30 VRAM, int4"},
}

func wizardPickQuantization(preset string, nonInteractive bool) (string, error) {
	if preset != "" {
		return preset, nil
	}
	if nonInteractive {
		return "auto", nil
	}
	options := make([]string, len(quantizationOptions))
	for i, q := range quantizationOptions {
		options[i] = q.desc
	}
	var picked string
	if err := survey.AskOne(&survey.Select{
		Message: "Quantization:",
		Options: options,
		Default: options[0],
	}, &picked); err != nil {
		return "", err
	}
	idx := indexOf(options, picked)
	if idx < 0 {
		return "auto", nil
	}
	return quantizationOptions[idx].id, nil
}

func normalizeQuantization(q string) string {
	if q == "" || q == "auto" {
		return ""
	}
	return q
}

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

func wizardPickProvider(presetProvider, presetMode string, advanced, nonInteractive bool) (mode string, provider string, err error) {
	if presetProvider != "" || presetMode == "manual" {
		return "manual", presetProvider, nil
	}
	if !advanced || nonInteractive {
		return "auto", "", nil
	}
	options := []string{
		"auto      smart routing (runpod → vastai → modal) — önerilen",
		"runpod    sadece RunPod",
		"vastai    sadece Vast.ai",
		"modal     sadece Modal",
	}
	var picked string
	if err := survey.AskOne(&survey.Select{
		Message: "Provider:",
		Options: options,
		Default: options[0],
	}, &picked); err != nil {
		return "", "", err
	}
	switch indexOf(options, picked) {
	case 1:
		return "manual", "runpod", nil
	case 2:
		return "manual", "vastai", nil
	case 3:
		return "manual", "modal", nil
	default:
		return "auto", "", nil
	}
}

// ---------------------------------------------------------------------------
// Idle / Budget prompts
// ---------------------------------------------------------------------------

func wizardAskInt(label, help string, preset, defaultVal int, nonInteractive bool) (int, error) {
	if preset > 0 {
		return preset, nil
	}
	if nonInteractive {
		return defaultVal, nil
	}
	var raw string
	if err := survey.AskOne(&survey.Input{
		Message: label + ":",
		Default: strconv.Itoa(defaultVal),
		Help:    help,
	}, &raw); err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return defaultVal, nil
	}
	return v, nil
}

func wizardAskFloat(label, help string, preset, defaultVal float64, nonInteractive bool) (float64, error) {
	if preset > 0 {
		return preset, nil
	}
	if nonInteractive {
		return defaultVal, nil
	}
	var raw string
	if err := survey.AskOne(&survey.Input{
		Message: label + ":",
		Default: fmt.Sprintf("%.2f", defaultVal),
		Help:    help,
	}, &raw); err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v <= 0 {
		return defaultVal, nil
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Preview + budget suggestion
// ---------------------------------------------------------------------------

func wizardPreview(parent context.Context, c *client.Client, hfID, gpu string) *client.DeploymentPreview {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	prev, err := c.PreviewDeployment(ctx, hfID, gpu)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  (preview alınamadı: %v)\n", err)
		return nil
	}
	return prev
}

// suggestBudget returns the user-supplied preset when set, otherwise picks a
// budget consistent with the gateway's preview hourly cost — one week of
// uninterrupted runtime plus 20% headroom — falling back to $50.
func suggestBudget(prev *client.DeploymentPreview, presetBudget float64) float64 {
	if presetBudget > 0 {
		return presetBudget
	}
	if prev != nil && prev.HourlyCostUSD > 0 {
		return roundCents(prev.HourlyCostUSD * 168 * 1.2)
	}
	return 50
}

func roundCents(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// ---------------------------------------------------------------------------
// Summary card
// ---------------------------------------------------------------------------

func printSummary(req client.CreateDeploymentRequest, prev *client.DeploymentPreview) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  ─────────────────────────────────────────────")
	fmt.Fprintln(os.Stderr, "  Özet")
	fmt.Fprintln(os.Stderr, "  ─────────────────────────────────────────────")
	fmt.Fprintf(os.Stderr, "  Model           %s\n", req.HuggingfaceID)
	fmt.Fprintf(os.Stderr, "  GPU             %s\n", req.GPUTier)
	fmt.Fprintf(os.Stderr, "  Quantization    %s\n", fallback(req.Quantization, "auto"))
	provider := req.Provider
	if provider == "" {
		provider = "auto (smart routing)"
	}
	fmt.Fprintf(os.Stderr, "  Provider        %s\n", provider)
	fmt.Fprintf(os.Stderr, "  Idle timeout    %d dk\n", req.IdleTimeoutMinutes)
	fmt.Fprintf(os.Stderr, "  Budget cap      $%.2f\n", req.BudgetLimitUSD)
	if prev != nil {
		fmt.Fprintln(os.Stderr, "  ─────────────────────────────────────────────")
		if prev.ETAMinutes > 0 {
			fmt.Fprintf(os.Stderr, "  ETA             ~%.0f dk\n", prev.ETAMinutes)
		}
		if prev.HourlyCostUSD > 0 {
			fmt.Fprintf(os.Stderr, "  Saatlik cost    $%.4f\n", prev.HourlyCostUSD)
		}
		cache := "soğuk (HF→S3 prefetch otomatik başlar)"
		if prev.IsCached {
			cache = "sıcak (S3 cache hit) — image_pulling ~1 dk"
		}
		fmt.Fprintf(os.Stderr, "  S3 cache        %s\n", cache)
	}
	fmt.Fprintln(os.Stderr, "  ─────────────────────────────────────────────")
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func indexOf(haystack []string, needle string) int {
	for i, v := range haystack {
		if v == needle {
			return i
		}
	}
	return -1
}

func defaultIfEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
