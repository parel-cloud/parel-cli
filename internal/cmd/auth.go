package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/config"
	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Parel CLI authentication",
}

var (
	authLoginNonInteractive bool
	authLoginAPIKeyFlag     string
	authLoginBaseURLFlag    string
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save an API key to the active profile",
	Long: `Saves a Parel API key to ` + "`auth.toml`" + ` after validating it against the
gateway. The key never leaves disk in plaintext logs and is stored with mode
0600.

Examples:
  parel auth login                                  # interactive prompt
  parel auth login --non-interactive --api-key pk-... --base-url https://api.parel.cloud`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()

		baseURL := authLoginBaseURLFlag
		if baseURL == "" {
			baseURL = flagBaseURL
		}
		if baseURL == "" {
			baseURL = client.DefaultBaseURL
		}

		apiKey := authLoginAPIKeyFlag
		if apiKey == "" && !authLoginNonInteractive {
			prompt := &survey.Password{
				Message: "Parel API key (paste; will not echo):",
				Help:    "Generate one at https://app.parel.cloud → API Keys",
			}
			if err := survey.AskOne(prompt, &apiKey, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			return fmt.Errorf("api key is required")
		}

		c := client.New(client.Options{APIKey: apiKey, BaseURL: baseURL, Version: version})
		whoami, err := c.Whoami(ctx)
		if err != nil {
			return printError(err)
		}

		f, err := config.Load()
		if err != nil {
			return err
		}
		f.Set(flagProfile, config.Profile{APIKey: apiKey, BaseURL: baseURL})
		if err := f.Save(); err != nil {
			return err
		}

		path, _ := config.Path()
		fmt.Fprintf(os.Stdout, "Saved profile %q to %s\n", flagProfile, path)
		fmt.Fprintf(os.Stdout, "Verified key against %s (active keys: %d)\n", baseURL, whoami.Total)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := config.Load()
		if err != nil {
			return err
		}
		removed := f.Delete(flagProfile)
		if !removed {
			fmt.Fprintf(os.Stdout, "Profile %q not found; nothing to remove\n", flagProfile)
			return nil
		}
		if err := f.Save(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Removed profile %q\n", flagProfile)
		return nil
	},
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Validate the active key and print the tenant + key prefix",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, profile, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()

		whoami, err := c.Whoami(ctx)
		if err != nil {
			return printError(err)
		}

		if flagJSON {
			return ui.PrintJSON(whoami)
		}

		fmt.Fprintf(os.Stdout, "profile:    %s\n", profile)
		fmt.Fprintf(os.Stdout, "base URL:   %s\n", c.BaseURL)
		fmt.Fprintf(os.Stdout, "active keys: %d\n", whoami.Total)
		if len(whoami.Data) == 0 {
			fmt.Fprintln(os.Stdout, "(no keys returned — admin may have revoked them)")
			return nil
		}
		t := ui.NewTable(os.Stdout)
		t.Headers("KEY PREFIX", "NAME", "ENV", "PII", "CREATED")
		for _, k := range whoami.Data {
			t.Row(k.KeyPrefix, k.Name, k.Env, k.PIIMode, k.CreatedAt.Format("2006-01-02"))
		}
		return t.Flush()
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show profile path, active profile and which env overrides apply",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.Path()
		if err != nil {
			return err
		}
		f, err := config.Load()
		if err != nil {
			return err
		}
		_, profileName := f.Resolve(flagProfile)
		fmt.Fprintf(os.Stdout, "auth.toml:        %s\n", path)
		fmt.Fprintf(os.Stdout, "default profile:  %s\n", f.DefaultProfile)
		fmt.Fprintf(os.Stdout, "active profile:   %s\n", profileName)
		profile := f.Profiles[profileName]
		fmt.Fprintf(os.Stdout, "  api_key:        %s\n", redactKey(profile.APIKey))
		fmt.Fprintf(os.Stdout, "  base_url:       %s\n", coalesce(profile.BaseURL, client.DefaultBaseURL))
		fmt.Fprintln(os.Stdout, "env overrides:")
		fmt.Fprintf(os.Stdout, "  PAREL_API_KEY:  %s\n", redactKey(os.Getenv("PAREL_API_KEY")))
		fmt.Fprintf(os.Stdout, "  PAREL_BASE_URL: %s\n", coalesce(os.Getenv("PAREL_BASE_URL"), "(unset)"))
		fmt.Fprintf(os.Stdout, "  PAREL_PROFILE:  %s\n", coalesce(os.Getenv("PAREL_PROFILE"), "(unset)"))
		return nil
	},
}

func redactKey(k string) string {
	k = strings.TrimSpace(k)
	if k == "" {
		return "(unset)"
	}
	if len(k) <= 8 {
		return "***"
	}
	return k[:4] + "…" + k[len(k)-4:]
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	authLoginCmd.Flags().BoolVar(&authLoginNonInteractive, "non-interactive", false, "skip prompts; require --api-key")
	authLoginCmd.Flags().StringVar(&authLoginAPIKeyFlag, "api-key", "", "API key to save (defaults to interactive prompt)")
	authLoginCmd.Flags().StringVar(&authLoginBaseURLFlag, "base-url", "", "gateway base URL (defaults to https://api.parel.cloud)")

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authWhoamiCmd, authStatusCmd)
	rootCmd.AddCommand(authCmd)
}
