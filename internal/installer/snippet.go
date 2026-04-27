// Package installer generates and writes the `claude-parel` shell launcher
// installed by `parel claude-code init`.
//
// The shell snippet keeps the env-var contract identical to
// web/src/components/connect-claude-code.tsx (function buildShellSetup,
// lines 155-215 in the parel monorepo). The CLI ships English comments;
// the parity test only enforces the literal env-var assignments.
package installer

import "strings"

// SnippetEnv mirrors the shape of the SnippetEnv interface in connect-claude-code.tsx.
type SnippetEnv struct {
	BaseURL   string // ANTHROPIC_BASE_URL value, typically https://api.parel.cloud/anthropic
	ModelID   string // Default Parel model id registered on the /model picker
	ModelName string // Display name shown in the picker
	APIKey    string // Optional: when non-empty, replaces "parel_..." placeholder
}

// PosixSnippet returns the zsh/bash launcher.
func PosixSnippet(env SnippetEnv) string {
	apiKey := env.APIKey
	if apiKey == "" {
		apiKey = "parel_..."
	}
	return strings.Join([]string{
		`# Parel launcher for Claude Code — default ` + "`claude`" + ` command stays untouched.`,
		`# Usage:        claude-parel`,
		`# Switch model: claude-parel -m qwen3.5-coder-32b`,
		`PAREL_MODEL_ID="` + env.ModelID + `"`,
		`PAREL_MODEL_NAME="` + env.ModelName + `"`,
		`PAREL_API_KEY="` + apiKey + `"   # app.parel.cloud > API Keys`,
		``,
		`claude-parel() {`,
		`  local model="$PAREL_MODEL_ID" name="$PAREL_MODEL_NAME"`,
		`  if [[ "$1" == "-m" || "$1" == "--model" ]]; then`,
		`    model="$2"; name="$2 (Parel)"; shift 2`,
		`  fi`,
		`  ANTHROPIC_BASE_URL="` + env.BaseURL + `" \`,
		`  ANTHROPIC_AUTH_TOKEN="$PAREL_API_KEY" \`,
		`  ANTHROPIC_API_KEY="" \`,
		`  ANTHROPIC_CUSTOM_MODEL_OPTION="$model" \`,
		`  ANTHROPIC_CUSTOM_MODEL_OPTION_NAME="$name" \`,
		`  ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION="Parel gateway - $model" \`,
		`  claude "$@"`,
		`}`,
	}, "\n")
}

// PowerShellSnippet returns the Windows launcher.
func PowerShellSnippet(env SnippetEnv) string {
	apiKey := env.APIKey
	if apiKey == "" {
		apiKey = "parel_..."
	}
	return strings.Join([]string{
		`# Parel launcher for Claude Code — default ` + "`claude`" + ` command stays untouched.`,
		`# Usage:        claude-parel`,
		`# Switch model: claude-parel -m qwen3.5-coder-32b`,
		`$PAREL_MODEL_ID = "` + env.ModelID + `"`,
		`$PAREL_MODEL_NAME = "` + env.ModelName + `"`,
		`$PAREL_API_KEY = "` + apiKey + `"   # app.parel.cloud > API Keys`,
		``,
		`function claude-parel {`,
		`  $model = $PAREL_MODEL_ID; $name = $PAREL_MODEL_NAME; $rest = @()`,
		`  for ($i = 0; $i -lt $args.Count; $i++) {`,
		`    if (($args[$i] -eq "-m" -or $args[$i] -eq "--model") -and ($i + 1 -lt $args.Count)) {`,
		`      $model = $args[$i + 1]; $name = "$($args[$i + 1]) (Parel)"; $i++`,
		`    } else { $rest += $args[$i] }`,
		`  }`,
		`  $saved = @{`,
		`    BASE = $env:ANTHROPIC_BASE_URL; AUTH = $env:ANTHROPIC_AUTH_TOKEN; KEY = $env:ANTHROPIC_API_KEY`,
		`    OPT = $env:ANTHROPIC_CUSTOM_MODEL_OPTION; NM = $env:ANTHROPIC_CUSTOM_MODEL_OPTION_NAME; DS = $env:ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION`,
		`  }`,
		`  try {`,
		`    $env:ANTHROPIC_BASE_URL = "` + env.BaseURL + `"`,
		`    $env:ANTHROPIC_AUTH_TOKEN = $PAREL_API_KEY`,
		`    $env:ANTHROPIC_API_KEY = ""`,
		`    $env:ANTHROPIC_CUSTOM_MODEL_OPTION = $model`,
		`    $env:ANTHROPIC_CUSTOM_MODEL_OPTION_NAME = $name`,
		`    $env:ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION = "Parel gateway - $model"`,
		`    claude @rest`,
		`  } finally {`,
		`    $env:ANTHROPIC_BASE_URL = $saved.BASE; $env:ANTHROPIC_AUTH_TOKEN = $saved.AUTH; $env:ANTHROPIC_API_KEY = $saved.KEY`,
		`    $env:ANTHROPIC_CUSTOM_MODEL_OPTION = $saved.OPT; $env:ANTHROPIC_CUSTOM_MODEL_OPTION_NAME = $saved.NM; $env:ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION = $saved.DS`,
		`  }`,
		`}`,
	}, "\n")
}
