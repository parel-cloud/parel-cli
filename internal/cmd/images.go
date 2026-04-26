package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Generate images via OpenAI-compatible /v1/images/generations",
}

var (
	imgModel  string
	imgN      int
	imgSize   string
	imgFormat string
	imgQuality string
	imgOut    string
	imgWait   bool
)

var imagesGenerateCmd = &cobra.Command{
	Use:   "generate <prompt>",
	Short: "Generate one or more images and save them to disk",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		req := client.ImageGenerateRequest{
			Model:          fallback(imgModel, "imagen-3"),
			Prompt:         strings.Join(args, " "),
			N:              imgN,
			Size:           imgSize,
			Quality:        imgQuality,
			ResponseFormat: imgFormat,
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		out, err := c.GenerateImage(ctx, req)
		if err != nil {
			return printError(err)
		}

		// Async response: a task_id is returned and we wait for the file URLs.
		if out.TaskID != "" {
			fmt.Fprintf(os.Stdout, "Image generation queued as task %s (status=%s)\n", out.TaskID, out.Status)
			if !imgWait {
				fmt.Fprintf(os.Stdout, "Wait for completion with: parel tasks show %s\n", out.TaskID)
				return nil
			}
			task, err := c.WaitForTask(cmd.Context(), out.TaskID, 3*time.Second, func(t *client.Task) {
				fmt.Fprintf(os.Stderr, "  status=%s\n", t.Status)
			})
			if err != nil {
				return printError(err)
			}
			if task.Status != "completed" {
				return fmt.Errorf("task %s ended with status=%s", task.ID, task.Status)
			}
			// Convert task.Result into ImageGenerateResponse-like shape.
			fmt.Fprintln(os.Stdout, "Task completed:")
			return ui.PrintRawJSON(task.Result)
		}

		if flagJSON {
			return ui.PrintJSON(out)
		}
		if len(out.Data) == 0 {
			fmt.Fprintln(os.Stdout, "(no images returned)")
			return nil
		}
		for i, img := range out.Data {
			path, err := saveImage(img, imgOut, i, len(out.Data))
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Saved: %s\n", path)
		}
		return nil
	},
}

func saveImage(img client.ImageObject, outFlag string, idx, total int) (string, error) {
	path := outFlag
	if path == "" {
		path = fmt.Sprintf("image-%d.png", idx+1)
	} else if total > 1 {
		// Suffix when caller asked for one path but model returned several.
		path = suffixPath(path, idx)
	}

	if img.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(img.B64JSON)
		if err != nil {
			return "", err
		}
		return path, os.WriteFile(path, data, 0o644)
	}
	if img.URL != "" {
		return path, downloadTo(img.URL, path)
	}
	return "", errors.New("image response has neither url nor b64_json")
}

func suffixPath(p string, idx int) string {
	dot := strings.LastIndex(p, ".")
	if dot < 0 {
		return fmt.Sprintf("%s-%d", p, idx+1)
	}
	return fmt.Sprintf("%s-%d%s", p[:dot], idx+1, p[dot:])
}

func downloadTo(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func init() {
	imagesGenerateCmd.Flags().StringVar(&imgModel, "model", "", "image model id (default: imagen-3)")
	imagesGenerateCmd.Flags().IntVar(&imgN, "n", 1, "number of images to generate")
	imagesGenerateCmd.Flags().StringVar(&imgSize, "size", "", "image size (e.g. 1024x1024)")
	imagesGenerateCmd.Flags().StringVar(&imgQuality, "quality", "", "model-specific quality knob")
	imagesGenerateCmd.Flags().StringVar(&imgFormat, "response-format", "url", "url or b64_json")
	imagesGenerateCmd.Flags().StringVarP(&imgOut, "out", "o", "", "output file (default: image-<n>.png)")
	imagesGenerateCmd.Flags().BoolVar(&imgWait, "wait", false, "wait for async tasks (queued models)")
	imagesCmd.AddCommand(imagesGenerateCmd)
	rootCmd.AddCommand(imagesCmd)
}
