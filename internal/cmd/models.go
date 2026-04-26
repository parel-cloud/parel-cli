package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List and inspect Parel catalog models (platform + instant + dedicated)",
}

var (
	modelsListType   string
	modelsListReady  bool
	modelsListSearch string
)

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Print the model catalog",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()

		out, err := c.ListModels(ctx)
		if err != nil {
			return printError(err)
		}

		filtered := filterModels(out.Data, modelsListType, modelsListReady, modelsListSearch)

		if flagJSON {
			return ui.PrintJSON(map[string]any{"data": filtered, "total": len(filtered)})
		}

		t := ui.NewTable(os.Stdout)
		t.Headers("ID", "TYPE", "SOURCE", "READY", "DISPLAY")
		for _, m := range filtered {
			ready := "yes"
			if m.IsReady != nil && !*m.IsReady {
				ready = m.NotReadyReason
				if ready == "" {
					ready = "no"
				}
			}
			t.Row(m.ID, modelType(m), nonEmpty(m.Source, "?"), ready, m.DisplayName)
		}
		if err := t.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "\n%d models\n", len(filtered))
		return nil
	},
}

var modelsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show one model with full pricing/capabilities/tier_access",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()

		m, err := c.GetModel(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(m)
		}

		fmt.Fprintf(os.Stdout, "id:           %s\n", m.ID)
		fmt.Fprintf(os.Stdout, "display:      %s\n", m.DisplayName)
		fmt.Fprintf(os.Stdout, "type:         %s\n", modelType(*m))
		fmt.Fprintf(os.Stdout, "provider:     %s\n", nonEmpty(m.Provider, "?"))
		fmt.Fprintf(os.Stdout, "source:       %s\n", nonEmpty(m.Source, "?"))
		fmt.Fprintf(os.Stdout, "status:       %s\n", nonEmpty(m.Status, "?"))
		ready := "yes"
		if m.IsReady != nil && !*m.IsReady {
			ready = "no"
			if m.NotReadyReason != "" {
				ready = ready + " (" + m.NotReadyReason + ")"
			}
		}
		fmt.Fprintf(os.Stdout, "ready:        %s\n", ready)
		if len(m.Badges) > 0 {
			fmt.Fprintf(os.Stdout, "badges:       %s\n", strings.Join(m.Badges, ", "))
		}
		if len(m.Pricing) > 0 {
			fmt.Fprintln(os.Stdout, "\npricing:")
			_ = ui.PrintRawJSON(m.Pricing)
		}
		if len(m.Capabilities) > 0 {
			fmt.Fprintln(os.Stdout, "\ncapabilities:")
			_ = ui.PrintRawJSON(m.Capabilities)
		}
		return nil
	},
}

func filterModels(in []client.Model, typeFilter string, readyOnly bool, search string) []client.Model {
	out := make([]client.Model, 0, len(in))
	typeFilter = strings.ToLower(strings.TrimSpace(typeFilter))
	search = strings.ToLower(strings.TrimSpace(search))
	for _, m := range in {
		if typeFilter != "" && !strings.Contains(strings.ToLower(modelType(m)), typeFilter) {
			continue
		}
		if readyOnly && m.IsReady != nil && !*m.IsReady {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(m.ID), search) && !strings.Contains(strings.ToLower(m.DisplayName), search) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func modelType(m client.Model) string {
	if m.ModelType != "" {
		return m.ModelType
	}
	return "?"
}

func nonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func init() {
	modelsListCmd.Flags().StringVar(&modelsListType, "type", "", "filter by model_type (llm, image, video, audio, embedding, ...)")
	modelsListCmd.Flags().BoolVar(&modelsListReady, "ready", false, "show only models with is_ready=true")
	modelsListCmd.Flags().StringVar(&modelsListSearch, "search", "", "case-insensitive substring match against id and display name")

	modelsCmd.AddCommand(modelsListCmd, modelsShowCmd)
	rootCmd.AddCommand(modelsCmd)
}
