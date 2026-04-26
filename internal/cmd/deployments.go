package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var deploymentsCmd = &cobra.Command{
	Use:     "deployments",
	Aliases: []string{"deploy"},
	Short:   "BYOM deployments (create, list, lifecycle, logs, billing)",
}

// ---------- create ----------

var (
	depCreateHFID         string
	depCreateName         string
	depCreateGPUTier      string
	depCreateProvider     string
	depCreateProviderMode string
	depCreateAllocation   string
	depCreateQuant        string
	depCreateIdleTimeout  int
	depCreateBudget       float64
	depCreateMaxModelLen  int
	depCreateMaxPrice     float64
	depCreateHFToken      string
	depCreateWait         bool
)

var deployCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a BYOM deployment from a HuggingFace model id",
	Long: `Submits a BYOM deployment request to the gateway. With --wait, polls until
the deployment reaches a terminal state (running / error / stopped).

Examples:
  parel deployments create --hf-id Qwen/Qwen2.5-7B-Instruct --gpu rtx-4090
  parel deployments create --hf-id Qwen/Qwen2.5-Coder-32B-Instruct --gpu h100_80gb \
    --quantization fp8 --idle-timeout 30 --budget 50 --max-price 1.50 --wait`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if depCreateHFID == "" {
			return errors.New("--hf-id is required")
		}
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
		defer cancel()

		req := client.CreateDeploymentRequest{
			HuggingfaceID:         depCreateHFID,
			Name:                  fallback(depCreateName, defaultDeploymentName(depCreateHFID)),
			GPUTier:               depCreateGPUTier,
			Provider:              depCreateProvider,
			ProviderMode:          depCreateProviderMode,
			AllocationMode:        depCreateAllocation,
			Quantization:          depCreateQuant,
			IdleTimeoutMinutes:    depCreateIdleTimeout,
			BudgetLimitUSD:        depCreateBudget,
			MaxModelLen:           depCreateMaxModelLen,
			ExpectedMaxPricePerHr: depCreateMaxPrice,
			HFToken:               depCreateHFToken,
		}
		dep, err := c.CreateDeployment(ctx, req)
		if err != nil {
			return printError(err)
		}

		if flagJSON && !depCreateWait {
			return ui.PrintJSON(dep)
		}
		fmt.Fprintf(os.Stdout, "Created deployment %s (status=%s, gpu=%s, provider=%s)\n",
			dep.ID, dep.Status, dep.GPULabel, dep.Provider)

		if !depCreateWait {
			fmt.Fprintf(os.Stdout, "\nPoll progress with: parel deployments events %s --follow\n", dep.ID)
			return nil
		}

		final, err := waitForDeployment(cmd.Context(), c, dep.ID, 5*time.Second)
		if err != nil {
			return err
		}
		if flagJSON {
			return ui.PrintJSON(final)
		}
		fmt.Fprintf(os.Stdout, "\nDeployment %s is %s\n", final.ID, final.Status)
		if final.Status == "running" {
			fmt.Fprintf(os.Stdout, "Model id: %s\n", final.ParelModelID)
		}
		if final.ErrorMessage != "" {
			fmt.Fprintf(os.Stdout, "Error: %s\n", final.ErrorMessage)
		}
		return nil
	},
}

// ---------- list ----------

var depListStatus string

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployments (optionally filtered by status)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()

		out, err := c.ListDeployments(ctx, depListStatus)
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		t := ui.NewTable(os.Stdout)
		t.Headers("ID", "STATUS", "MODEL", "GPU", "PROVIDER", "AGE")
		for _, d := range out.Data {
			t.Row(short(d.ID), d.Status, d.HuggingfaceID, d.GPULabel, d.Provider, ageOf(d.CreatedAt))
		}
		if err := t.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "\n%d deployments\n", len(out.Data))
		return nil
	},
}

// ---------- show ----------

var deployShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show one deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()

		dep, err := c.GetDeployment(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(dep)
		}
		printDeployment(dep)
		return nil
	},
}

// ---------- start / stop / rm ----------

var deployStartCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "Wake or restart a stopped/idle deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		dep, err := c.StartDeployment(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(dep)
		}
		fmt.Fprintf(os.Stdout, "Started %s (status=%s)\n", dep.ID, dep.Status)
		return nil
	},
}

var deployStopCmd = &cobra.Command{
	Use:   "stop <id>",
	Short: "Stop a running deployment (final billing recorded)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		dep, err := c.StopDeployment(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(dep)
		}
		fmt.Fprintf(os.Stdout, "Stopped %s (status=%s, last_request=%s)\n", dep.ID, dep.Status, fallback(dep.LastRequestAt, "—"))
		return nil
	},
}

var depRMForce bool

var deployRMCmd = &cobra.Command{
	Use:     "rm <id>",
	Aliases: []string{"delete"},
	Short:   "Delete a deployment (terminates the GPU pod)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		if !depRMForce {
			fmt.Fprintf(os.Stdout, "Delete deployment %s? Type 'yes' to confirm: ", args[0])
			var ans string
			fmt.Fscanln(os.Stdin, &ans)
			if strings.ToLower(strings.TrimSpace(ans)) != "yes" {
				fmt.Fprintln(os.Stdout, "Aborted")
				return nil
			}
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		out, err := c.DeleteDeployment(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		fmt.Fprintf(os.Stdout, "Deleted %s\n", out.ID)
		return nil
	},
}

// ---------- events ----------

var depEventsFollow bool
var depEventsSince string

var deployEventsCmd = &cobra.Command{
	Use:   "events <id>",
	Short: "Tail deployment events",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		seen := make(map[string]bool)
		printOnce := func(ctx context.Context) error {
			out, err := c.ListDeploymentEvents(ctx, args[0], depEventsSince)
			if err != nil {
				return printError(err)
			}
			for _, ev := range out.Data {
				if seen[ev.ID] {
					continue
				}
				seen[ev.ID] = true
				fmt.Fprintf(os.Stdout, "%s  %-12s  %s\n", ev.CreatedAt, ev.Type, ev.Message)
			}
			return nil
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		err = printOnce(ctx)
		cancel()
		if err != nil {
			return err
		}
		if !depEventsFollow {
			return nil
		}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-cmd.Context().Done():
				return nil
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
				err := printOnce(ctx)
				cancel()
				if err != nil {
					return err
				}
			}
		}
	},
}

// ---------- metrics + billing ----------

var deployMetricsCmd = &cobra.Command{
	Use:   "metrics <id>",
	Short: "Print live deployment metrics (last 5 minutes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.DeploymentMetrics(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		return ui.PrintRawJSON(out)
	},
}

var deployBillingCmd = &cobra.Command{
	Use:   "billing <id>",
	Short: "Print recorded GPU billing for one deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.DeploymentBilling(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		return ui.PrintRawJSON(out)
	},
}

// ---------- preview / validate-hf / templates / gpu-tiers ----------

var (
	previewHFID string
	previewGPU  string
)

var deployPreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Estimate ETA + hourly cost without spinning up a pod",
	RunE: func(cmd *cobra.Command, args []string) error {
		if previewHFID == "" {
			return errors.New("--hf-id is required")
		}
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		p, err := c.PreviewDeployment(ctx, previewHFID, previewGPU)
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(p)
		}
		fmt.Fprintf(os.Stdout, "model:    %s\n", p.HuggingfaceID)
		fmt.Fprintf(os.Stdout, "gpu_tier: %s\n", p.GPUTier)
		fmt.Fprintf(os.Stdout, "provider: %s\n", fallback(p.Provider, "auto"))
		fmt.Fprintf(os.Stdout, "ETA:      %.1f min\n", p.ETAMinutes)
		fmt.Fprintf(os.Stdout, "cost:     $%.4f/hr\n", p.HourlyCostUSD)
		fmt.Fprintf(os.Stdout, "cached:   %v\n", p.IsCached)
		if p.CacheStatus != "" {
			fmt.Fprintf(os.Stdout, "cache:    %s\n", p.CacheStatus)
		}
		if p.Notes != "" {
			fmt.Fprintf(os.Stdout, "note:     %s\n", p.Notes)
		}
		return nil
	},
}

var deployValidateHFCmd = &cobra.Command{
	Use:   "validate-hf <hf-id>",
	Short: "Run HF validator (architecture, VRAM, vLLM compat) against a model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		out, err := c.ValidateHF(ctx, client.HFValidateRequest{ModelID: args[0]})
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		fmt.Fprintf(os.Stdout, "valid:           %v\n", out.Valid)
		fmt.Fprintf(os.Stdout, "architecture:    %s\n", fallback(out.Architecture, "—"))
		fmt.Fprintf(os.Stdout, "vram (fp16):     %.1f GB\n", out.VRAMFp16GB)
		fmt.Fprintf(os.Stdout, "vram (int4):     %.1f GB\n", out.VRAMInt4GB)
		fmt.Fprintf(os.Stdout, "recommended GPU: %s\n", fallback(out.RecommendedGPUTier, "—"))
		fmt.Fprintf(os.Stdout, "ghost test:      %v\n", out.GhostTestPassed)
		if !out.GhostTestPassed && out.GhostTestSignature != "" {
			fmt.Fprintf(os.Stdout, "ghost signature: %s\n", out.GhostTestSignature)
		}
		if out.NeedsNewVLLM {
			fmt.Fprintln(os.Stdout, "note: model needs a newer vLLM; provider compat may be limited")
		}
		if len(out.Errors) > 0 {
			fmt.Fprintln(os.Stdout, "errors:")
			for _, e := range out.Errors {
				fmt.Fprintf(os.Stdout, "  - %s\n", e)
			}
		}
		return nil
	},
}

var deployTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List built-in BYOM deployment templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.ListDeploymentTemplates(ctx)
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		t := ui.NewTable(os.Stdout)
		t.Headers("ID", "NAME", "HF MODEL", "GPU", "DESCRIPTION")
		for _, tmpl := range out.Data {
			t.Row(tmpl.ID, tmpl.Name, tmpl.HuggingfaceID, tmpl.GPUTier, tmpl.Description)
		}
		return t.Flush()
	},
}

// ---------- gpu sub-group ----------

var gpuCmd = &cobra.Command{
	Use:   "gpu",
	Short: "GPU tier catalogue + live capacity",
}

var gpuTiersLive bool

var gpuTiersCmd = &cobra.Command{
	Use:   "tiers",
	Short: "List GPU tiers (use --live for capacity check)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		out, err := c.ListGPUTiers(ctx, gpuTiersLive)
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		t := ui.NewTable(os.Stdout)
		t.Headers("ID", "NAME", "VRAM", "PRICE/HR", "PROVIDER", "CAPACITY")
		for _, tier := range out.Data {
			t.Row(tier.ID, tier.DisplayName, fmt.Sprintf("%dGB", tier.VRAMGB),
				fmt.Sprintf("$%.4f", tier.PricePerHrUSD), fallback(tier.Provider, "—"), fallback(tier.Capacity, "—"))
		}
		return t.Flush()
	},
}

// ---------- helpers ----------

func waitForDeployment(parent context.Context, c *client.Client, id string, interval time.Duration) (*client.Deployment, error) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		ctx, cancel := context.WithTimeout(parent, 15*time.Second)
		dep, err := c.GetDeployment(ctx, id)
		cancel()
		if err != nil {
			return nil, printError(err)
		}
		fmt.Fprintf(os.Stdout, "  status=%s\n", dep.Status)
		switch dep.Status {
		case "running", "error", "failed", "stopped", "deleted":
			return dep, nil
		}
		select {
		case <-parent.Done():
			return dep, parent.Err()
		case <-time.After(interval):
		}
	}
}

func printDeployment(d *client.Deployment) {
	fmt.Fprintf(os.Stdout, "id:              %s\n", d.ID)
	fmt.Fprintf(os.Stdout, "name:            %s\n", d.Name)
	fmt.Fprintf(os.Stdout, "status:          %s\n", d.Status)
	fmt.Fprintf(os.Stdout, "huggingface_id:  %s\n", d.HuggingfaceID)
	fmt.Fprintf(os.Stdout, "parel_model_id:  %s\n", d.ParelModelID)
	fmt.Fprintf(os.Stdout, "gpu:             %s (%s)\n", d.GPULabel, d.GPUTier)
	fmt.Fprintf(os.Stdout, "provider:        %s\n", d.Provider)
	fmt.Fprintf(os.Stdout, "quantization:    %s\n", d.Quantization)
	fmt.Fprintf(os.Stdout, "idle_timeout:    %d min\n", d.IdleTimeoutMinutes)
	fmt.Fprintf(os.Stdout, "budget:          $%.2f / $%.2f spent\n", d.BudgetSpentUSD, d.BudgetLimitUSD)
	fmt.Fprintf(os.Stdout, "hourly_cost:     $%.4f\n", d.HourlyCost)
	fmt.Fprintf(os.Stdout, "invoke_base_url: %s\n", fallback(d.InvokeBaseURL, "—"))
	fmt.Fprintf(os.Stdout, "created:         %s\n", d.CreatedAt)
	if d.StartedAt != "" {
		fmt.Fprintf(os.Stdout, "started:         %s\n", d.StartedAt)
	}
	if d.StoppedAt != "" {
		fmt.Fprintf(os.Stdout, "stopped:         %s\n", d.StoppedAt)
	}
	if d.LastRequestAt != "" {
		fmt.Fprintf(os.Stdout, "last_request:    %s\n", d.LastRequestAt)
	}
	if d.ErrorMessage != "" {
		fmt.Fprintf(os.Stdout, "error:           %s\n", d.ErrorMessage)
	}
	if d.ETAAvgMinutes > 0 {
		fmt.Fprintf(os.Stdout, "eta:             %.1f min (n=%d, %s)\n", d.ETAAvgMinutes, d.ETASampleCount, d.ETABasis)
	}
}

func defaultDeploymentName(hfID string) string {
	parts := strings.Split(hfID, "/")
	leaf := parts[len(parts)-1]
	leaf = strings.ToLower(leaf)
	leaf = strings.NewReplacer("_", "-", ".", "-").Replace(leaf)
	if leaf == "" {
		leaf = "deployment"
	}
	return leaf
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func ageOf(iso string) string {
	if iso == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func fallback(v, alt string) string {
	if v == "" {
		return alt
	}
	return v
}

func init() {
	deployCreateCmd.Flags().StringVar(&depCreateHFID, "hf-id", "", "HuggingFace model id (required)")
	deployCreateCmd.Flags().StringVar(&depCreateName, "name", "", "deployment name (default: derived from hf-id)")
	deployCreateCmd.Flags().StringVar(&depCreateGPUTier, "gpu", "", "GPU tier id (e.g. rtx-4090, h100_80gb)")
	deployCreateCmd.Flags().StringVar(&depCreateProvider, "provider", "", "force a single provider (runpod / vastai / modal)")
	deployCreateCmd.Flags().StringVar(&depCreateProviderMode, "provider-mode", "", "manual or auto (smart routing)")
	deployCreateCmd.Flags().StringVar(&depCreateAllocation, "allocation", "", "exact or auto allocation mode")
	deployCreateCmd.Flags().StringVar(&depCreateQuant, "quantization", "", "fp16 / fp8 / awq / gptq (default: HF auto)")
	deployCreateCmd.Flags().IntVar(&depCreateIdleTimeout, "idle-timeout", 15, "minutes of idle before sleep")
	deployCreateCmd.Flags().Float64Var(&depCreateBudget, "budget", 100.0, "USD budget cap")
	deployCreateCmd.Flags().IntVar(&depCreateMaxModelLen, "max-model-len", 8192, "vLLM max_model_len")
	deployCreateCmd.Flags().Float64Var(&depCreateMaxPrice, "max-price", 0, "expected_max_price_per_hr (price-confirmation gate)")
	deployCreateCmd.Flags().StringVar(&depCreateHFToken, "hf-token", "", "HuggingFace token for gated models")
	deployCreateCmd.Flags().BoolVar(&depCreateWait, "wait", false, "poll until deployment reaches a terminal status")

	deployListCmd.Flags().StringVar(&depListStatus, "status", "", "filter by status (running, stopped, error, ...)")

	deployRMCmd.Flags().BoolVarP(&depRMForce, "force", "f", false, "skip confirmation prompt")

	deployEventsCmd.Flags().BoolVarP(&depEventsFollow, "follow", "f", false, "tail new events every 2 seconds")
	deployEventsCmd.Flags().StringVar(&depEventsSince, "since", "", "ISO timestamp; only events newer than this")

	deployPreviewCmd.Flags().StringVar(&previewHFID, "hf-id", "", "HuggingFace model id (required)")
	deployPreviewCmd.Flags().StringVar(&previewGPU, "gpu", "", "GPU tier id (optional; default auto)")

	gpuTiersCmd.Flags().BoolVar(&gpuTiersLive, "live", false, "include live provider capacity (slower)")

	deploymentsCmd.AddCommand(
		deployCreateCmd, deployListCmd, deployShowCmd,
		deployStartCmd, deployStopCmd, deployRMCmd,
		deployEventsCmd, deployMetricsCmd, deployBillingCmd,
		deployPreviewCmd, deployValidateHFCmd, deployTemplatesCmd,
	)
	gpuCmd.AddCommand(gpuTiersCmd)

	rootCmd.AddCommand(deploymentsCmd, gpuCmd)
}
