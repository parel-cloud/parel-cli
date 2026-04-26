package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version, commit, build date and platform",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("parel %s\n", version)
		fmt.Printf("  commit:   %s\n", commit)
		fmt.Printf("  built:    %s\n", date)
		fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  go:       %s\n", runtime.Version())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
