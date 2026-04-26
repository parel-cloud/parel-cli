package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/installer"
	"github.com/spf13/cobra"
)

var claudeCodeCmd = &cobra.Command{
	Use:   "claude-code",
	Short: "Install / inspect / remove the claude-parel shell launcher",
}

var (
	ccInitNonInteractive bool
	ccInitModel          string
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

		// Pick the default model.
		modelID := ccInitModel
		if modelID == "" && !ccInitNonInteractive {
			pickedID, pickedDisplay, err := pickDefaultModel(cmd.Context(), apiKey)
			if err != nil {
				return err
			}
			modelID = pickedID
			_ = pickedDisplay
		}
		if modelID == "" {
			modelID = "qwen3-max"
		}
		modelName := modelID + " (Parel)"

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
		fmt.Fprintf(os.Stdout, "profile: %s\n", shell.Path)
		if block == "" {
			fmt.Fprintln(os.Stdout, "no parel claude-code block installed (run `parel claude-code init`)")
			return nil
		}
		fmt.Fprintln(os.Stdout, "---")
		fmt.Fprintln(os.Stdout, block)
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

func pickDefaultModel(parent context.Context, apiKey string) (string, string, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	c := client.New(client.Options{APIKey: apiKey, BaseURL: flagBaseURL, Version: version})
	models, err := c.ListModels(ctx)
	if err != nil {
		return "", "", printError(err)
	}
	candidates := filterClaudeCodeCandidates(models.Data)
	if len(candidates) == 0 {
		return "qwen3-max", "qwen3-max", nil
	}
	options := make([]string, len(candidates))
	for i, m := range candidates {
		label := m.ID
		if m.DisplayName != "" && m.DisplayName != m.ID {
			label = fmt.Sprintf("%s — %s", m.ID, m.DisplayName)
		}
		options[i] = label
	}
	var picked string
	prompt := &survey.Select{
		Message: "Pick the default Parel model for /model picker:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &picked); err != nil {
		return "", "", err
	}
	id := strings.SplitN(picked, " — ", 2)[0]
	for _, m := range candidates {
		if m.ID == id {
			return m.ID, m.DisplayName, nil
		}
	}
	return id, id, nil
}

func filterClaudeCodeCandidates(in []client.Model) []client.Model {
	out := make([]client.Model, 0, len(in))
	for _, m := range in {
		if strings.HasPrefix(m.ID, "claude-") || strings.HasPrefix(m.ID, "anthropic/") {
			continue
		}
		if m.ModelType != "" && !strings.Contains(strings.ToLower(m.ModelType), "llm") &&
			!strings.Contains(strings.ToLower(m.ModelType), "chat") {
			continue
		}
		if m.IsReady != nil && !*m.IsReady {
			continue
		}
		out = append(out, m)
	}
	return out
}

func init() {
	claudeCodeInitCmd.Flags().BoolVar(&ccInitNonInteractive, "non-interactive", false, "skip prompts (use defaults)")
	claudeCodeInitCmd.Flags().StringVar(&ccInitModel, "model", "", "default Parel model id (skip picker)")
	claudeCodeInitCmd.Flags().StringVar(&ccInitAPIKey, "api-key", "", "Parel API key (skip prompt)")
	claudeCodeInitCmd.Flags().StringVar(&ccInitProfilePath, "profile-path", "", "override profile file path")

	claudeCodeCmd.AddCommand(claudeCodeInitCmd, claudeCodeStatusCmd, claudeCodeUninstallCmd)
	rootCmd.AddCommand(claudeCodeCmd)
}
