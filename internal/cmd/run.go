package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	runInputs []string
	runOut    string
	runStream bool
)

var runCmd = &cobra.Command{
	Use:   "run <model>",
	Short: "Dispatch wrapper: pick chat / images / videos / audio based on the model id",
	Long: `One unified entry point for ad-hoc inference. Provide model + named inputs and
the wrapper dispatches to the right /v1/* endpoint, saving file output when
applicable.

Examples:
  parel run qwen3-max -i prompt="Write a haiku" --stream
  parel run imagen-3 -i prompt="cyberpunk cat" -o cat.png
  parel run fal-veo-2 -i prompt="rain forest aerial" --wait -o forest.mp4
  parel run whisper-large-v3 -i file=audio.m4a`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelID := args[0]
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		inputs, err := parseRunInputs(runInputs)
		if err != nil {
			return err
		}

		// We can't see model metadata without a /v1/models call; sniff from
		// the inputs the user provided.
		switch {
		case inputs["file"] != "":
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			out, err := c.Transcribe(ctx, inputs["file"], client.TranscribeOptions{Model: modelID})
			if err != nil {
				return printError(err)
			}
			if flagJSON {
				return ui.PrintJSON(out)
			}
			fmt.Fprintln(os.Stdout, out.Text)
			return nil

		case strings.HasPrefix(strings.ToLower(modelID), "tts") || inputs["voice"] != "":
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			body, ctype, err := c.Speak(ctx, client.SpeechRequest{
				Model: modelID,
				Input: inputs["prompt"],
				Voice: inputs["voice"],
			})
			if err != nil {
				return printError(err)
			}
			path := runOut
			if path == "" {
				path = "tts" + extensionForContentType(ctype)
			}
			if err := os.WriteFile(path, body, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Saved: %s\n", path)
			return nil

		case looksLikeVideoModel(modelID):
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			out, err := c.GenerateVideo(ctx, client.VideoGenerateRequest{
				Model:  modelID,
				Prompt: inputs["prompt"],
			})
			if err != nil {
				return printError(err)
			}
			fmt.Fprintf(os.Stdout, "Queued task %s\n", out.TaskID)
			return nil

		case looksLikeImageModel(modelID):
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			out, err := c.GenerateImage(ctx, client.ImageGenerateRequest{
				Model:  modelID,
				Prompt: inputs["prompt"],
				N:      1,
				Size:   inputs["size"],
			})
			if err != nil {
				return printError(err)
			}
			if out.TaskID != "" {
				fmt.Fprintf(os.Stdout, "Queued task %s (poll with `parel tasks show %s`)\n", out.TaskID, out.TaskID)
				return nil
			}
			if len(out.Data) == 0 {
				return errors.New("no images returned")
			}
			path := runOut
			if path == "" {
				path = "image.png"
			}
			actual, err := saveImage(out.Data[0], path, 0, 1)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Saved: %s\n", actual)
			return nil

		default:
			req := client.ChatRequest{
				Model:    modelID,
				Messages: []client.ChatMessage{client.TextMessage("user", inputs["prompt"])},
				Stream:   runStream,
			}
			if sys := inputs["system"]; sys != "" {
				req.Messages = append([]client.ChatMessage{client.TextMessage("system", sys)}, req.Messages...)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			if runStream {
				_, err := streamOrOnce(ctx, c, req, true)
				fmt.Fprintln(os.Stdout)
				return err
			}
			resp, err := c.Chat(ctx, req)
			if err != nil {
				return printError(err)
			}
			if flagJSON {
				return ui.PrintJSON(resp)
			}
			if len(resp.Choices) == 0 {
				fmt.Fprintln(os.Stdout, "(no choices)")
				return nil
			}
			var content string
			if err := json.Unmarshal(resp.Choices[0].Message.Content, &content); err != nil {
				_, _ = os.Stdout.Write(resp.Choices[0].Message.Content)
				fmt.Fprintln(os.Stdout)
				return nil
			}
			fmt.Fprintln(os.Stdout, content)
			return nil
		}
	},
}

func parseRunInputs(in []string) (map[string]string, error) {
	out := map[string]string{}
	for _, kv := range in {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("invalid -i %q (expected key=value)", kv)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		out[key] = val
	}
	return out, nil
}

func looksLikeImageModel(id string) bool {
	id = strings.ToLower(id)
	return strings.Contains(id, "imagen") ||
		strings.Contains(id, "flux") ||
		strings.Contains(id, "dall") ||
		strings.Contains(id, "ideogram") ||
		strings.Contains(id, "recraft") ||
		strings.Contains(id, "stable-diffusion")
}

func looksLikeVideoModel(id string) bool {
	id = strings.ToLower(id)
	return strings.Contains(id, "veo") ||
		strings.Contains(id, "kling") ||
		strings.Contains(id, "seedance") ||
		strings.Contains(id, "video")
}

func init() {
	runCmd.Flags().StringSliceVarP(&runInputs, "input", "i", nil, "named input (key=value); repeat for multiple")
	runCmd.Flags().StringVarP(&runOut, "out", "o", "", "output file for image/video/audio results")
	runCmd.Flags().BoolVar(&runStream, "stream", false, "stream chat completions")
	rootCmd.AddCommand(runCmd)
}
