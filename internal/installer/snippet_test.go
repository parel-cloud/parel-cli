package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleEnv() SnippetEnv {
	return SnippetEnv{
		BaseURL:   "https://api.parel.cloud/anthropic",
		ModelID:   "qwen3-max",
		ModelName: "qwen3-max (Parel)",
	}
}

func TestPosixSnippet_ContainsRequiredEnvVars(t *testing.T) {
	out := PosixSnippet(sampleEnv())
	required := []string{
		`PAREL_MODEL_ID="qwen3-max"`,
		`PAREL_MODEL_NAME="qwen3-max (Parel)"`,
		`ANTHROPIC_BASE_URL="https://api.parel.cloud/anthropic"`,
		`ANTHROPIC_AUTH_TOKEN="$PAREL_API_KEY"`,
		`ANTHROPIC_API_KEY=""`,
		`ANTHROPIC_CUSTOM_MODEL_OPTION="$model"`,
		`ANTHROPIC_CUSTOM_MODEL_OPTION_NAME="$name"`,
		`ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION="Parel gateway - $model"`,
		`claude-parel()`,
		`if [ -f "$HOME/.parel/claude-code.disabled" ]; then`,
		`command claude "$@"; return`,
		`claude "$@"`,
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Errorf("POSIX snippet missing %q", want)
		}
	}
}

// TestPosixSnippet_MarkerCheckIsFirstStatement guards the order: the marker
// short-circuit must run before any Parel env var assignment. Otherwise
// `claude-parel` would still leak ANTHROPIC_* vars even when disabled.
func TestPosixSnippet_MarkerCheckIsFirstStatement(t *testing.T) {
	out := PosixSnippet(sampleEnv())
	markerIdx := strings.Index(out, `claude-code.disabled`)
	envIdx := strings.Index(out, `ANTHROPIC_BASE_URL=`)
	if markerIdx < 0 || envIdx < 0 {
		t.Fatalf("snippet missing expected lines: %q", out)
	}
	if markerIdx > envIdx {
		t.Errorf("marker check must precede ANTHROPIC_BASE_URL assignment; got marker@%d > env@%d", markerIdx, envIdx)
	}
}

func TestPowerShellSnippet_ContainsRequiredEnvVars(t *testing.T) {
	out := PowerShellSnippet(sampleEnv())
	required := []string{
		`$PAREL_MODEL_ID = "qwen3-max"`,
		`$PAREL_MODEL_NAME = "qwen3-max (Parel)"`,
		`$env:ANTHROPIC_BASE_URL = "https://api.parel.cloud/anthropic"`,
		`$env:ANTHROPIC_API_KEY = ""`,
		`$env:ANTHROPIC_CUSTOM_MODEL_OPTION = $model`,
		`function claude-parel`,
		`if (Test-Path "$HOME/.parel/claude-code.disabled") {`,
		`& claude @args; return`,
		`claude @rest`,
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Errorf("PowerShell snippet missing %q", want)
		}
	}
}

func TestSnippet_AcceptsExplicitAPIKey(t *testing.T) {
	env := sampleEnv()
	env.APIKey = "pk-test-123"
	out := PosixSnippet(env)
	if !strings.Contains(out, `PAREL_API_KEY="pk-test-123"`) {
		t.Errorf("explicit APIKey not propagated: %q", out)
	}
	if strings.Contains(out, `parel_...`) {
		t.Errorf("placeholder leaked into snippet with explicit key: %q", out)
	}
}

// TestSnippet_ParityWithMonorepo asserts the CLI snippet stays in lock-step
// with web/src/components/connect-claude-code.tsx (function buildShellSetup).
//
// Full byte-equivalence isn't practical because TS template literals escape
// backslashes and backticks differently than Go string literals, so this
// instead confirms the canonical key lines all appear in both sources.
func TestSnippet_ParityWithMonorepo(t *testing.T) {
	root := os.Getenv("PAREL_MONOREPO")
	if root == "" {
		t.Skip("PAREL_MONOREPO not set; skipping parity check")
	}
	tsFile := filepath.Join(root, "web", "src", "components", "connect-claude-code.tsx")
	src, err := os.ReadFile(tsFile)
	if err != nil {
		t.Skipf("cannot read %s: %v", tsFile, err)
	}
	tsSrc := string(src)

	// Lines that must appear verbatim in the TS source so the launcher we ship
	// matches what the web UI advertises. If this diverges, both surfaces are
	// out of sync and users will hit conflicting setup instructions.
	posixCanon := []string{
		`PAREL_MODEL_ID="${env.modelId}"`,
		`PAREL_API_KEY="parel_..."`,
		`ANTHROPIC_BASE_URL="${env.baseUrl}"`,
		`ANTHROPIC_AUTH_TOKEN="$PAREL_API_KEY"`,
		`ANTHROPIC_API_KEY=""`,
		`ANTHROPIC_CUSTOM_MODEL_OPTION="$model"`,
		`ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION="Parel gateway - $model"`,
		`if [ -f "$HOME/.parel/claude-code.disabled" ]; then`,
		`command claude "$@"; return`,
		`claude "$@"`,
	}
	pwshCanon := []string{
		`$PAREL_MODEL_ID = "${env.modelId}"`,
		`$env:ANTHROPIC_BASE_URL = "${env.baseUrl}"`,
		`$env:ANTHROPIC_API_KEY = ""`,
		`$env:ANTHROPIC_CUSTOM_MODEL_OPTION = $model`,
		`function claude-parel`,
		`if (Test-Path "$HOME/.parel/claude-code.disabled") {`,
		`& claude @args; return`,
		`claude @rest`,
	}
	for _, want := range posixCanon {
		if !strings.Contains(tsSrc, want) {
			t.Errorf("monorepo TS source missing POSIX canon: %q", want)
		}
	}
	for _, want := range pwshCanon {
		if !strings.Contains(tsSrc, want) {
			t.Errorf("monorepo TS source missing PowerShell canon: %q", want)
		}
	}
}
