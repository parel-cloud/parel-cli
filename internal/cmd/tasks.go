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

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Inspect async generation tasks (image, video, ...)",
}

var (
	tasksListType   string
	tasksListStatus string
	tasksListLimit  int
)

var tasksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.ListTasks(ctx, client.ListTasksOptions{
			TaskType: tasksListType,
			Status:   tasksListStatus,
			Limit:    tasksListLimit,
		})
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		t := ui.NewTable(os.Stdout)
		t.Headers("ID", "TYPE", "STATUS", "MODEL", "AGE")
		for _, task := range out.Data {
			t.Row(short(task.ID), task.TaskType, task.Status, task.Model, ageOf(task.CreatedAt))
		}
		if err := t.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "\n%d tasks\n", len(out.Data))
		return nil
	},
}

var tasksShowFollow bool

var tasksShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show one task (use --follow to poll until terminal)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		if tasksShowFollow {
			task, err := c.WaitForTask(cmd.Context(), args[0], 3*time.Second, func(t *client.Task) {
				fmt.Fprintf(os.Stderr, "  status=%s progress=%.0f%%\n", t.Status, t.Progress*100)
			})
			if err != nil {
				return printError(err)
			}
			return ui.PrintJSON(task)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		task, err := c.GetTask(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		return ui.PrintJSON(task)
	},
}

var tasksCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel a running task and refund (idempotent)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		task, err := c.CancelTask(ctx, args[0])
		if err != nil {
			return printError(err)
		}
		fmt.Fprintf(os.Stdout, "Task %s status=%s\n", task.ID, task.Status)
		return nil
	},
}

func init() {
	tasksListCmd.Flags().StringVar(&tasksListType, "type", "", "filter by task_type (image, video, ...)")
	tasksListCmd.Flags().StringVar(&tasksListStatus, "status", "", "filter by status (pending, processing, completed, failed, cancelled)")
	tasksListCmd.Flags().IntVar(&tasksListLimit, "limit", 0, "max items to return")

	tasksShowCmd.Flags().BoolVarP(&tasksShowFollow, "follow", "f", false, "poll until terminal status")

	tasksCmd.AddCommand(tasksListCmd, tasksShowCmd, tasksCancelCmd)
	rootCmd.AddCommand(tasksCmd)
}
