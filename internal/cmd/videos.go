package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var videosCmd = &cobra.Command{
	Use:   "videos",
	Short: "Generate video via async tasks",
}

var (
	vidModel    string
	vidDuration int
	vidSize     string
	vidImageURL string
	vidOut      string
	vidWait     bool
)

var videosGenerateCmd = &cobra.Command{
	Use:   "generate <prompt>",
	Short: "Submit a video generation task and optionally wait for output",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		req := client.VideoGenerateRequest{
			Model:    fallback(vidModel, "fal-veo-2"),
			Prompt:   strings.Join(args, " "),
			Duration: vidDuration,
			Size:     vidSize,
			ImageURL: vidImageURL,
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		out, err := c.GenerateVideo(ctx, req)
		if err != nil {
			return printError(err)
		}
		fmt.Fprintf(os.Stdout, "Video task queued: %s (status=%s)\n", out.TaskID, out.Status)
		if !vidWait {
			fmt.Fprintf(os.Stdout, "Poll with: parel tasks show %s\n", out.TaskID)
			return nil
		}
		task, err := c.WaitForTask(cmd.Context(), out.TaskID, 5*time.Second, func(t *client.Task) {
			fmt.Fprintf(os.Stderr, "  status=%s progress=%.0f%%\n", t.Status, t.Progress*100)
		})
		if err != nil {
			return printError(err)
		}
		if task.Status != "completed" {
			return fmt.Errorf("task %s ended with status=%s", task.ID, task.Status)
		}

		// task.Result usually carries {"video_url": "https://..."}
		var result struct {
			VideoURL string `json:"video_url"`
			URL      string `json:"url"`
		}
		_ = json.Unmarshal(task.Result, &result)
		url := firstNonEmpty(result.VideoURL, result.URL)
		if url == "" {
			fmt.Fprintln(os.Stdout, "Result:")
			return ui.PrintRawJSON(task.Result)
		}
		path := vidOut
		if path == "" {
			path = "video.mp4"
		}
		if err := downloadTo(url, path); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Saved: %s (%s)\n", path, url)
		return nil
	},
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	videosGenerateCmd.Flags().StringVar(&vidModel, "model", "", "video model id (default: fal-veo-2)")
	videosGenerateCmd.Flags().IntVar(&vidDuration, "duration", 0, "duration seconds (model-dependent)")
	videosGenerateCmd.Flags().StringVar(&vidSize, "size", "", "frame size, e.g. 1280x720")
	videosGenerateCmd.Flags().StringVar(&vidImageURL, "image-url", "", "image-to-video reference frame")
	videosGenerateCmd.Flags().StringVarP(&vidOut, "out", "o", "video.mp4", "output file")
	videosGenerateCmd.Flags().BoolVar(&vidWait, "wait", false, "block until task completes and download the result")

	videosCmd.AddCommand(videosGenerateCmd)
	rootCmd.AddCommand(videosCmd)
}
