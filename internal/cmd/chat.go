package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	chatModel       string
	chatStream      bool
	chatSystem      string
	chatTemperature float64
	chatMaxTokens   int
)

var chatCmd = &cobra.Command{
	Use:   "chat [prompt|-]",
	Short: "One-shot chat completion (use - to read prompt from stdin)",
	Long: `Sends a single chat completion. With --stream, prints tokens as they arrive.

Examples:
  parel chat "Merhaba" --model qwen3-max --stream
  echo "Bana bir haiku yaz" | parel chat - --model gpt-5.4
  parel chat repl --model qwen3-max`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("prompt is required (use '-' to read from stdin or `parel chat repl`)")
		}
		prompt, err := readPrompt(args[0])
		if err != nil {
			return err
		}
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		req := chatRequest(prompt)
		if chatStream {
			return runChatStream(cmd.Context(), c, req)
		}
		return runChatOnce(cmd.Context(), c, req)
	},
}

var chatReplCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start an interactive chat REPL",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "parel chat REPL — model=%s. Type /exit, /reset, or /system <prompt> to control.\n",
			fallback(chatModel, "qwen3-max"))

		reader := bufio.NewReader(os.Stdin)
		messages := []client.ChatMessage{}
		if chatSystem != "" {
			messages = append(messages, client.TextMessage("system", chatSystem))
		}
		for {
			fmt.Fprint(os.Stdout, "> ")
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			switch {
			case line == "/exit", line == "/quit":
				return nil
			case line == "/reset":
				messages = messages[:0]
				if chatSystem != "" {
					messages = append(messages, client.TextMessage("system", chatSystem))
				}
				fmt.Fprintln(os.Stdout, "(history cleared)")
				continue
			case strings.HasPrefix(line, "/system "):
				sys := strings.TrimSpace(strings.TrimPrefix(line, "/system "))
				messages = append([]client.ChatMessage{client.TextMessage("system", sys)}, messages...)
				fmt.Fprintln(os.Stdout, "(system prompt updated)")
				continue
			}
			messages = append(messages, client.TextMessage("user", line))
			req := client.ChatRequest{
				Model:    fallback(chatModel, "qwen3-max"),
				Messages: messages,
				Stream:   chatStream,
			}
			if chatTemperature > 0 {
				v := chatTemperature
				req.Temperature = &v
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			full, err := streamOrOnce(ctx, c, req, chatStream)
			cancel()
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				continue
			}
			messages = append(messages, client.TextMessage("assistant", full))
			fmt.Fprintln(os.Stdout)
		}
	},
}

func chatRequest(prompt string) client.ChatRequest {
	messages := []client.ChatMessage{}
	if chatSystem != "" {
		messages = append(messages, client.TextMessage("system", chatSystem))
	}
	messages = append(messages, client.TextMessage("user", prompt))
	req := client.ChatRequest{
		Model:    fallback(chatModel, "qwen3-max"),
		Messages: messages,
		Stream:   chatStream,
	}
	if chatTemperature > 0 {
		v := chatTemperature
		req.Temperature = &v
	}
	if chatMaxTokens > 0 {
		v := chatMaxTokens
		req.MaxTokens = &v
	}
	return req
}

func runChatOnce(parent context.Context, c *client.Client, req client.ChatRequest) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
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
		// Multi-modal content: just dump raw JSON.
		_, _ = os.Stdout.Write(resp.Choices[0].Message.Content)
		fmt.Fprintln(os.Stdout)
		return nil
	}
	fmt.Fprintln(os.Stdout, content)
	if resp.Usage.TotalTokens > 0 && flagVerbose > 0 {
		fmt.Fprintf(os.Stderr, "\n[%d prompt + %d completion = %d tokens]\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}
	return nil
}

func runChatStream(parent context.Context, c *client.Client, req client.ChatRequest) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	_, err := streamOrOnce(ctx, c, req, true)
	fmt.Fprintln(os.Stdout)
	return err
}

// streamOrOnce drives a streaming or non-streaming chat call and prints tokens
// to stdout. Returns the assembled assistant text so the REPL can append it
// to the running history.
func streamOrOnce(ctx context.Context, c *client.Client, req client.ChatRequest, stream bool) (string, error) {
	if !stream {
		resp, err := c.Chat(ctx, req)
		if err != nil {
			return "", printError(err)
		}
		if len(resp.Choices) == 0 {
			return "", nil
		}
		var text string
		if err := json.Unmarshal(resp.Choices[0].Message.Content, &text); err != nil {
			text = string(resp.Choices[0].Message.Content)
		}
		fmt.Fprint(os.Stdout, text)
		return text, nil
	}

	events, errs := c.ChatStream(ctx, req)
	var collected strings.Builder
	for ev := range events {
		if ev.IsDone() {
			continue
		}
		// OpenAI streaming chunk shape: choices[0].delta.content
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(ev.Data, &chunk); err != nil {
			continue
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content == "" {
				continue
			}
			fmt.Fprint(os.Stdout, ch.Delta.Content)
			collected.WriteString(ch.Delta.Content)
		}
	}
	if err := <-errs; err != nil {
		return collected.String(), printError(err)
	}
	return collected.String(), nil
}

func readPrompt(arg string) (string, error) {
	if arg != "-" {
		return arg, nil
	}
	buf, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(buf), "\n"), nil
}

func init() {
	chatCmd.Flags().StringVar(&chatModel, "model", "", "model id (default: qwen3-max)")
	chatCmd.Flags().BoolVar(&chatStream, "stream", false, "print tokens as they arrive")
	chatCmd.Flags().StringVar(&chatSystem, "system", "", "optional system prompt")
	chatCmd.Flags().Float64Var(&chatTemperature, "temperature", 0, "sampling temperature (0 = default)")
	chatCmd.Flags().IntVar(&chatMaxTokens, "max-tokens", 0, "max completion tokens (0 = provider default)")

	chatReplCmd.Flags().StringVar(&chatModel, "model", "", "model id (default: qwen3-max)")
	chatReplCmd.Flags().BoolVar(&chatStream, "stream", true, "stream tokens as they arrive")
	chatReplCmd.Flags().StringVar(&chatSystem, "system", "", "optional system prompt")
	chatReplCmd.Flags().Float64Var(&chatTemperature, "temperature", 0, "sampling temperature (0 = default)")

	chatCmd.AddCommand(chatReplCmd)
	rootCmd.AddCommand(chatCmd)
}
