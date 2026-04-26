package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagProfile  string
	flagAPIKey   string
	flagBaseURL  string
	flagJSON     bool
	flagNoColor  bool
	flagVerbose  int
)

const longDescription = `Parel CLI is a single binary for the Parel AI gateway.

Run BYOM (bring-your-own-model) deployments, list catalog models, run chat /
images / video / audio inference, and bridge any OpenAI- or Anthropic-compatible
client to Parel via the local proxy server.

Documentation: https://parel.cloud
Source:        https://github.com/parel-cloud/parel-cli`

var rootCmd = &cobra.Command{
	Use:           "parel",
	Short:         "Parel CLI - single binary for the Parel AI gateway",
	Long:          longDescription,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", envOr("PAREL_PROFILE", "default"), "config profile name")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", os.Getenv("PAREL_API_KEY"), "Parel API key (overrides profile)")
	rootCmd.PersistentFlags().StringVar(&flagBaseURL, "base-url", envOr("PAREL_BASE_URL", "https://api.parel.cloud"), "gateway base URL")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "machine-readable JSON output")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", os.Getenv("NO_COLOR") != "", "disable colored output")
	rootCmd.PersistentFlags().CountVarP(&flagVerbose, "verbose", "v", "verbose output (-vv enables HTTP wire trace)")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
