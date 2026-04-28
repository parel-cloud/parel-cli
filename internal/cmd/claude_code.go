package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/installer"
	"github.com/spf13/cobra"
)

// disableMarkerPath returns the marker file the shell snippet checks at runtime.
// When this file exists, `claude-parel` falls through to the unmodified `claude`
// binary, which lets the user dodge a broken Parel custom model without having
// to edit the profile or restart their shell.
func disableMarkerPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".parel", "claude-code.disabled"), nil
}

func disableMarkerExists() (bool, string, error) {
	path, err := disableMarkerPath()
	if err != nil {
		return false, "", err
	}
	if _, err := os.Stat(path); err == nil {
		return true, path, nil
	} else if !os.IsNotExist(err) {
		return false, path, err
	}
	return false, path, nil
}

var claudeCodeCmd = &cobra.Command{
	Use:   "claude-code",
	Short: "Install / inspect / remove the claude-parel shell launcher",
}

var (
	ccInitNonInteractive bool
	ccInitModel          string
	ccInitModelName      string
	ccInitAPIKey         string
	ccInitProfilePath    string
)

const defaultAnthropicBase = "https://api.parel.cloud/anthropic"

var claudeCodeInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Install the claude-parel function into your shell profile",
	Long: `Writes a managed block (idempotent, marker-bounded) into your shell profile
that defines a ` + "`claude-parel`" + ` launcher function. Inside the launcher:

  ANTHROPIC_BASE_URL=` + defaultAnthropicBase + `
  ANTHROPIC_AUTH_TOKEN=<your Parel key>
  ANTHROPIC_API_KEY=""                             (must stay empty)
  ANTHROPIC_CUSTOM_MODEL_OPTION=<chosen model>     (registers in /model picker)

The default ` + "`claude`" + ` command is left untouched. Use ` + "`claude-parel -m <id>`" + `
to switch model in-session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		shell, err := installer.DetectShell()
		if err != nil {
			return err
		}
		if ccInitProfilePath != "" {
			shell.Path = ccInitProfilePath
		}

		// Resolve the API key from flags, profile, or interactive prompt.
		apiKey := ccInitAPIKey
		if apiKey == "" {
			if c, _, err := resolveClient(); err == nil {
				apiKey = c.APIKey
			}
		}
		if apiKey == "" && !ccInitNonInteractive {
			prompt := &survey.Password{Message: "Parel API key:"}
			if err := survey.AskOne(prompt, &apiKey, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}
		if apiKey == "" {
			return fmt.Errorf("api key is required (run `parel auth login` first or pass --api-key)")
		}

		// Resolve model id + human-readable name. Precedence for the name:
		//   1. --model-name flag (explicit override)
		//   2. interactive picker's display_name (from the gateway model row)
		//   3. /v1/models/<id> lookup when --model was passed non-interactively
		//   4. fallback: "<id> (Parel)"
		modelID := ccInitModel
		modelName := ccInitModelName
		if modelID == "" && !ccInitNonInteractive {
			pickedID, pickedDisplay, err := pickDefaultModel(cmd.Context(), apiKey)
			if err != nil {
				return err
			}
			modelID = pickedID
			if modelName == "" {
				modelName = pickedDisplay
			}
		}
		if modelID == "" {
			modelID = "qwen3-max"
		}
		if modelName == "" {
			modelName = lookupModelDisplayName(cmd.Context(), apiKey, modelID)
		}
		if modelName == "" || modelName == modelID {
			modelName = modelID + " (Parel)"
		}

		var snippet string
		env := installer.SnippetEnv{
			BaseURL:   defaultAnthropicBase,
			ModelID:   modelID,
			ModelName: modelName,
			APIKey:    apiKey,
		}
		if shell.IsPowerShell() {
			snippet = installer.PowerShellSnippet(env)
		} else {
			snippet = installer.PosixSnippet(env)
		}

		opts := installer.WriteOptions{
			Shell:      shell,
			Snippet:    snippet,
			CLIVersion: version,
		}
		changed, err := installer.Apply(shell.Path, opts)
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintf(os.Stdout, "Updated %s (model: %s, base: %s)\n", shell.Path, modelID, defaultAnthropicBase)
		} else {
			fmt.Fprintf(os.Stdout, "%s already up to date\n", shell.Path)
		}

		if _, err := exec.LookPath("claude"); err != nil {
			fmt.Fprintln(os.Stderr, "warning: `claude` not found in PATH. Install Claude Code: https://github.com/anthropics/claude-code")
		}
		fmt.Fprintf(os.Stdout, "\n%s\n", shell.SourceCommand())
		return nil
	},
}

var claudeCodeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the currently installed claude-parel block",
	RunE: func(cmd *cobra.Command, args []string) error {
		shell, err := installer.DetectShell()
		if err != nil {
			return err
		}
		block, err := installer.Inspect(shell.Path)
		if err != nil {
			return err
		}
		disabled, markerPath, err := disableMarkerExists()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "profile: %s\n", shell.Path)
		if disabled {
			fmt.Fprintf(os.Stdout, "state:   DISABLED (marker: %s)\n", markerPath)
			fmt.Fprintln(os.Stdout, "         claude-parel falls through to plain `claude`. Run `parel claude-code enable` to restore.")
		} else {
			fmt.Fprintln(os.Stdout, "state:   enabled")
		}
		if block == "" {
			fmt.Fprintln(os.Stdout, "no parel claude-code block installed (run `parel claude-code init`)")
			return nil
		}
		fmt.Fprintln(os.Stdout, "---")
		fmt.Fprintln(os.Stdout, block)
		return nil
	},
}

var claudeCodeDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Temporarily route claude-parel to plain `claude` (skip Parel custom model)",
	Long: `Creates a marker file at ~/.parel/claude-code.disabled. The shell snippet
checks this on every invocation, so claude-parel falls through to the plain
` + "`claude`" + ` binary without injecting any Parel env vars or the custom model option.

Useful when auto / agentic mode breaks because the chosen Parel model is
misbehaving. Re-enable with ` + "`parel claude-code enable`" + `. Effect is instant,
no shell restart needed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := disableMarkerPath()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := fmt.Fprintf(f, "disabled by `parel claude-code disable` at %s\n", time.Now().UTC().Format(time.RFC3339)); err != nil {
			return err
		}

		// Comment out the managed block so a fresh shell doesn't define
		// PAREL_* vars or the claude-parel function. Belt-and-suspenders with
		// the marker: marker is instant for already-running shells, the
		// commented block prevents new shells from re-introducing the env.
		shell, shellErr := installer.DetectShell()
		blockTouched := false
		if shellErr == nil {
			if block, _ := installer.Inspect(shell.Path); block == "" {
				fmt.Fprintf(os.Stderr, "note: no parel claude-code block found in %s. Run `parel claude-code init` to install the launcher.\n", shell.Path)
			} else {
				if !strings.Contains(block, "claude-code.disabled") {
					fmt.Fprintln(os.Stderr, "note: your installed snippet is older and does not check the disable marker. Run `parel claude-code init` to refresh.")
				}
				if changed, err := installer.SetBlockDisabled(shell.Path, true); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not comment out the managed block in %s: %v\n", shell.Path, err)
				} else {
					blockTouched = changed
				}
			}
		}

		fmt.Fprintf(os.Stdout, "Disabled. claude-parel falls through to plain `claude` immediately (marker: %s).\n", path)
		if blockTouched {
			fmt.Fprintf(os.Stdout, "The managed block in %s is now commented out. Run `source %s` (or open a new terminal) so plain `claude` no longer sees the Parel custom model.\n", shell.Path, shell.ProfileFmt)
		}
		return nil
	},
}

var claudeCodeEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Re-enable the Parel custom model option in claude-parel",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := disableMarkerPath()
		if err != nil {
			return err
		}
		markerExisted := true
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				markerExisted = false
			} else {
				return err
			}
		}

		// Uncomment the managed block so a fresh shell re-defines PAREL_*
		// vars and the claude-parel function.
		shell, shellErr := installer.DetectShell()
		blockTouched := false
		if shellErr == nil {
			if block, _ := installer.Inspect(shell.Path); block != "" {
				if changed, err := installer.SetBlockDisabled(shell.Path, false); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not uncomment the managed block in %s: %v\n", shell.Path, err)
				} else {
					blockTouched = changed
				}
			}
		}

		switch {
		case markerExisted && blockTouched:
			fmt.Fprintf(os.Stdout, "Enabled. claude-parel routes through Parel again. Run `source %s` (or open a new terminal) so plain `claude` regains the Parel custom model.\n", shell.ProfileFmt)
		case blockTouched:
			fmt.Fprintf(os.Stdout, "Enabled. The managed block was re-activated. Run `source %s` (or open a new terminal) so plain `claude` regains the Parel custom model.\n", shell.ProfileFmt)
		case markerExisted:
			fmt.Fprintln(os.Stdout, "Enabled. claude-parel routes through Parel again.")
		default:
			fmt.Fprintln(os.Stdout, "Already enabled (no disable marker present).")
		}
		return nil
	},
}

var claudeCodeUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the parel claude-code block from your shell profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		shell, err := installer.DetectShell()
		if err != nil {
			return err
		}
		removed, err := installer.Remove(shell.Path)
		if err != nil {
			return err
		}
		if removed {
			fmt.Fprintf(os.Stdout, "Removed parel claude-code block from %s\n", shell.Path)
		} else {
			fmt.Fprintf(os.Stdout, "No parel claude-code block found in %s\n", shell.Path)
		}
		return nil
	},
}

// modelGroup labels a curated source bucket for the hierarchical picker.
type modelGroup struct {
	kind    string // "dedicated" | "instant" | "platform"
	label   string
	hint    string
	models  []client.Model
}

// lookupModelDisplayName resolves a human-readable name for modelID by hitting
// the gateway. Best-effort: any error or empty display_name returns "" so the
// caller can fall back to "<id> (Parel)" without failing the install.
//
// Resolution order:
//   1. BYOM (id starts with "byom-"): /v1/deployments/<uuid> → display_name|name|huggingface_id
//   2. /v1/models/<id> (single fetch, works for platform + instant tm_ models)
//   3. /v1/models list scan (covers tenant-scoped rows that 404 on the single GET)
func lookupModelDisplayName(parent context.Context, apiKey, modelID string) string {
	if modelID == "" || apiKey == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	c := client.New(client.Options{APIKey: apiKey, BaseURL: flagBaseURL, Version: version})

	if strings.HasPrefix(modelID, "byom-") {
		depID := strings.TrimPrefix(modelID, "byom-")
		if dep, err := c.GetDeployment(ctx, depID); err == nil && dep != nil {
			if dep.DisplayName != "" {
				return dep.DisplayName
			}
			if dep.Name != "" {
				return dep.Name
			}
			if dep.HuggingfaceID != "" {
				return dep.HuggingfaceID
			}
		}
	}

	if m, err := c.GetModel(ctx, modelID); err == nil && m != nil && m.DisplayName != "" {
		return m.DisplayName
	}
	if list, err := c.ListModels(ctx); err == nil && list != nil {
		for _, m := range list.Data {
			if m.ID == modelID && m.DisplayName != "" {
				return m.DisplayName
			}
		}
	}
	return ""
}

func pickDefaultModel(parent context.Context, apiKey string) (string, string, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	c := client.New(client.Options{APIKey: apiKey, BaseURL: flagBaseURL, Version: version})
	models, err := c.ListModels(ctx)
	if err != nil {
		return "", "", printError(err)
	}
	groups := groupCandidates(models.Data)
	nonEmpty := make([]modelGroup, 0, 3)
	for _, g := range groups {
		if len(g.models) > 0 {
			nonEmpty = append(nonEmpty, g)
		}
	}
	if len(nonEmpty) == 0 {
		fmt.Fprintln(os.Stderr, "warning: no chat-capable Parel models found; defaulting to qwen3-max")
		return "qwen3-max", "qwen3-max", nil
	}

	// Choose the group: skip the prompt when there's only one non-empty bucket.
	var chosen modelGroup
	if len(nonEmpty) == 1 {
		chosen = nonEmpty[0]
	} else {
		groupOpts := make([]string, len(nonEmpty))
		for i, g := range nonEmpty {
			groupOpts[i] = fmt.Sprintf("%s  (%d)  — %s", g.label, len(g.models), g.hint)
		}
		var picked string
		gp := &survey.Select{
			Message: "Which models would you like to choose from?",
			Options: groupOpts,
			Help:    "BYOM = your own rented GPU. Instant = your imported HuggingFace tenant models. Showcase = Parel's hosted catalog.",
		}
		if err := survey.AskOne(gp, &picked); err != nil {
			return "", "", err
		}
		for i, opt := range groupOpts {
			if opt == picked {
				chosen = nonEmpty[i]
				break
			}
		}
	}

	options := make([]string, len(chosen.models))
	byLabel := make(map[string]client.Model, len(chosen.models))
	for i, m := range chosen.models {
		options[i] = formatPickerLabel(m)
		byLabel[options[i]] = m
	}
	var picked string
	prompt := &survey.Select{
		Message: fmt.Sprintf("Pick a model from %s:", chosen.label),
		Options: options,
	}
	if err := survey.AskOne(prompt, &picked); err != nil {
		return "", "", err
	}
	if m, ok := byLabel[picked]; ok {
		return m.ID, m.DisplayName, nil
	}
	id := strings.TrimPrefix(strings.SplitN(picked, " — ", 2)[0], "[BYOM] ")
	id = strings.TrimPrefix(id, "[Instant] ")
	return id, id, nil
}

// groupCandidates returns the three buckets in the canonical onboarding order:
// the user's own dedicated BYOM deployments first, then their imported HF
// instant models, then the Parel showcase / platform catalog.
func groupCandidates(in []client.Model) []modelGroup {
	candidates := filterClaudeCodeCandidates(in)
	g := []modelGroup{
		{kind: "dedicated", label: "My own GPUs (BYOM)", hint: "your rented deployments"},
		{kind: "instant", label: "My imported models", hint: "HuggingFace tenant models"},
		{kind: "platform", label: "Parel showcase", hint: "qwen3-max, gpt-5.4, deepseek-v3.2 ..."},
	}
	for _, m := range candidates {
		switch {
		case strings.HasPrefix(m.ID, "byom-") || strings.EqualFold(m.Source, "dedicated"):
			g[0].models = append(g[0].models, m)
		case strings.HasPrefix(m.ID, "tm_") || strings.EqualFold(m.Source, "instant"):
			g[1].models = append(g[1].models, m)
		default:
			g[2].models = append(g[2].models, m)
		}
	}
	return g
}

// filterClaudeCodeCandidates picks the models that make sense as Claude Code's
// brain and orders them: dedicated BYOM (the user's own GPU) first, then
// imported HF instant tenant models, then platform/showcase models. Anthropic-
// native models are filtered out because the gateway's /anthropic/v1/messages
// proxy explicitly rejects them, and visual modalities (image/video/audio) are
// dropped because Claude Code wants chat models.
func filterClaudeCodeCandidates(in []client.Model) []client.Model {
	var dedicated, instant, platform []client.Model
	for _, m := range in {
		if strings.HasPrefix(m.ID, "claude-") || strings.HasPrefix(m.ID, "anthropic/") {
			continue
		}
		// Drop strictly-not-text modalities. BYOM models often have empty
		// model_type so we keep them; we only filter when the type is
		// definitively non-text.
		if mt := strings.ToLower(m.ModelType); mt != "" {
			if strings.Contains(mt, "image") || strings.Contains(mt, "video") ||
				strings.Contains(mt, "audio") || strings.Contains(mt, "tts") ||
				strings.Contains(mt, "stt") || strings.Contains(mt, "embed") {
				continue
			}
		}
		if m.IsReady != nil && !*m.IsReady {
			continue
		}

		switch {
		case strings.HasPrefix(m.ID, "byom-") || strings.EqualFold(m.Source, "dedicated"):
			dedicated = append(dedicated, m)
		case strings.HasPrefix(m.ID, "tm_") || strings.EqualFold(m.Source, "instant"):
			instant = append(instant, m)
		default:
			platform = append(platform, m)
		}
	}

	out := make([]client.Model, 0, len(dedicated)+len(instant)+len(platform))
	out = append(out, dedicated...)
	out = append(out, instant...)
	out = append(out, platform...)
	return out
}

// formatPickerLabel renders a one-line picker entry with a source rosette so
// the user can tell BYOM (their own GPU) from instant API or platform models
// at a glance.
func formatPickerLabel(m client.Model) string {
	var rosette string
	switch {
	case strings.HasPrefix(m.ID, "byom-") || strings.EqualFold(m.Source, "dedicated"):
		rosette = "[BYOM] "
	case strings.HasPrefix(m.ID, "tm_") || strings.EqualFold(m.Source, "instant"):
		rosette = "[Instant] "
	}
	display := m.DisplayName
	if display == "" || display == m.ID {
		return rosette + m.ID
	}
	return fmt.Sprintf("%s%s — %s", rosette, m.ID, display)
}

func init() {
	claudeCodeInitCmd.Flags().BoolVar(&ccInitNonInteractive, "non-interactive", false, "skip prompts (use defaults)")
	claudeCodeInitCmd.Flags().StringVar(&ccInitModel, "model", "", "default Parel model id (skip picker)")
	claudeCodeInitCmd.Flags().StringVar(&ccInitModelName, "model-name", "", "display name shown in the /model picker (default: gateway display_name, falls back to \"<id> (Parel)\")")
	claudeCodeInitCmd.Flags().StringVar(&ccInitAPIKey, "api-key", "", "Parel API key (skip prompt)")
	claudeCodeInitCmd.Flags().StringVar(&ccInitProfilePath, "profile-path", "", "override profile file path")

	claudeCodeCmd.AddCommand(claudeCodeInitCmd, claudeCodeStatusCmd, claudeCodeDisableCmd, claudeCodeEnableCmd, claudeCodeUninstallCmd)
	rootCmd.AddCommand(claudeCodeCmd)
}
