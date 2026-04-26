package cmd

import (
	"context"
	"time"

	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Account usage, budget, spending, GPU billing",
}

var (
	usageFrom string
	usageTo   string
	usageBy   string
)

var usageSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Spend summary across providers and models",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.UsageSummary(ctx, usageFrom, usageTo)
		if err != nil {
			return printError(err)
		}
		return ui.PrintRawJSON(out)
	},
}

var usageBudgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Remaining credits and budget caps",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.UsageBudget(ctx)
		if err != nil {
			return printError(err)
		}
		return ui.PrintRawJSON(out)
	},
}

var usageSpendingCmd = &cobra.Command{
	Use:   "spending",
	Short: "Itemised spending (--by provider | model)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.UsageSpending(ctx, usageBy)
		if err != nil {
			return printError(err)
		}
		return ui.PrintRawJSON(out)
	},
}

var usageGPUBillingCmd = &cobra.Command{
	Use:   "gpu-billing",
	Short: "Tenant GPU billing records (BYOM)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.UsageGPUBilling(ctx)
		if err != nil {
			return printError(err)
		}
		return ui.PrintRawJSON(out)
	},
}

func init() {
	usageSummaryCmd.Flags().StringVar(&usageFrom, "from", "", "start date YYYY-MM-DD")
	usageSummaryCmd.Flags().StringVar(&usageTo, "to", "", "end date YYYY-MM-DD")
	usageSpendingCmd.Flags().StringVar(&usageBy, "by", "", "group by provider | model")

	usageCmd.AddCommand(usageSummaryCmd, usageBudgetCmd, usageSpendingCmd, usageGPUBillingCmd)
	rootCmd.AddCommand(usageCmd)
}
