package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/parel-cloud/parel-cli/internal/proxy"
	"github.com/spf13/cobra"
)

var (
	proxyPort     int
	proxyBind     string
	proxyUpstream string
	proxyQuiet    bool
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run a local OpenAI- and Anthropic-compatible HTTP server backed by Parel",
	Long: `Starts a tiny HTTP server on 127.0.0.1:7878 (configurable). Any client that
expects an OpenAI- or Anthropic-style endpoint can target localhost; the proxy
forwards to the Parel gateway with your API key already attached. The point:
your IDE config never holds a Parel key.

Routes:
  POST /v1/chat/completions
  POST /anthropic/v1/messages
  POST /anthropic/v1/messages/count_tokens
  GET  /healthz

Bind Claude Code to it:
  ANTHROPIC_BASE_URL=http://127.0.0.1:7878/anthropic ANTHROPIC_AUTH_TOKEN=any claude

Pin a BYOM deployment:
  parel proxy --upstream byom-<uuid>     # rewrites the request body's model field`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		srv, err := proxy.New(proxy.Options{
			BaseURL:  c.BaseURL,
			APIKey:   c.APIKey,
			Bind:     proxyBind,
			Port:     proxyPort,
			Upstream: proxyUpstream,
		})
		if err != nil {
			return err
		}
		if !proxyQuiet {
			srv.SetLogger(func(format string, args ...any) {
				fmt.Fprintf(os.Stderr, format+"\n", args...)
			})
		}

		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		fmt.Fprintf(os.Stdout, "parel proxy listening on http://%s\n", srv.Address())
		fmt.Fprintf(os.Stdout, "  upstream: %s\n", c.BaseURL)
		if proxyUpstream != "" {
			fmt.Fprintf(os.Stdout, "  pinned model: %s (request bodies rewritten)\n", proxyUpstream)
		}
		fmt.Fprintln(os.Stdout, "  routes:")
		fmt.Fprintln(os.Stdout, "    POST /v1/chat/completions")
		fmt.Fprintln(os.Stdout, "    POST /anthropic/v1/messages")
		fmt.Fprintln(os.Stdout, "    POST /anthropic/v1/messages/count_tokens")
		fmt.Fprintln(os.Stdout, "    GET  /healthz")
		fmt.Fprintln(os.Stdout, "Press Ctrl-C to stop.")

		if err := srv.Start(ctx); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	proxyCmd.Flags().IntVar(&proxyPort, "port", 7878, "listen port")
	proxyCmd.Flags().StringVar(&proxyBind, "bind", "127.0.0.1", "bind address")
	proxyCmd.Flags().StringVar(&proxyUpstream, "upstream", "", "force-rewrite model field to this BYOM id (e.g. byom-<uuid>)")
	proxyCmd.Flags().BoolVar(&proxyQuiet, "quiet", false, "suppress per-request log lines on stderr")

	rootCmd.AddCommand(proxyCmd)

	// silence the unused-context import lint for go 1.20- style.
	_ = context.Background
}
