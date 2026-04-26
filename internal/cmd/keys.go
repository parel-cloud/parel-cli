package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage Parel API keys for the active tenant",
}

var (
	keyCreateName     string
	keyCreateEnv      string
	keyCreatePIIMode  string
	keyCreateProtLvl  string
)

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Mint a new API key (max 10 active per tenant)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if keyCreateName == "" {
			return fmt.Errorf("--name is required")
		}
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.CreateAPIKey(ctx, client.CreateAPIKeyRequest{
			Name:            keyCreateName,
			Env:             keyCreateEnv,
			PIIMode:         keyCreatePIIMode,
			ProtectionLevel: keyCreateProtLvl,
		})
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		fmt.Fprintln(os.Stdout, "API key created (this is the ONLY time the secret is shown):")
		fmt.Fprintf(os.Stdout, "  id:         %s\n", out.ID)
		fmt.Fprintf(os.Stdout, "  key:        %s\n", out.Key)
		fmt.Fprintf(os.Stdout, "  prefix:     %s\n", out.KeyPrefix)
		fmt.Fprintf(os.Stdout, "  name:       %s\n", out.Name)
		fmt.Fprintf(os.Stdout, "  env:        %s\n", out.Env)
		fmt.Fprintf(os.Stdout, "  pii_mode:   %s\n", out.PIIMode)
		fmt.Fprintf(os.Stdout, "  protection: %s\n", out.ProtectionLevel)
		return nil
	},
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active API keys for the current tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.ListAPIKeys(ctx)
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		t := ui.NewTable(os.Stdout)
		t.Headers("PREFIX", "NAME", "ENV", "PII", "CREATED", "LAST USED")
		for _, k := range out.Data {
			lastUsed := "never"
			if k.LastUsedAt != nil {
				lastUsed = ageOf(k.LastUsedAt.Format(time.RFC3339))
			}
			t.Row(k.KeyPrefix, k.Name, k.Env, k.PIIMode, k.CreatedAt.Format("2006-01-02"), lastUsed)
		}
		return t.Flush()
	},
}

var keysRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an API key by id (auth cache flushed immediately)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.RevokeAPIKey(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		fmt.Fprintf(os.Stdout, "Revoked %s\n", out.ID)
		return nil
	},
}

func init() {
	keysCreateCmd.Flags().StringVar(&keyCreateName, "name", "", "human-readable label (required)")
	keysCreateCmd.Flags().StringVar(&keyCreateEnv, "env", "", "prod | staging | dev")
	keysCreateCmd.Flags().StringVar(&keyCreatePIIMode, "pii-mode", "", "off | mask | block")
	keysCreateCmd.Flags().StringVar(&keyCreateProtLvl, "protection-level", "", "standard | strict")

	keysCmd.AddCommand(keysCreateCmd, keysListCmd, keysRevokeCmd)
	rootCmd.AddCommand(keysCmd)
}
