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

var (
	embModel string
	embRaw   bool
)

var embeddingsCmd = &cobra.Command{
	Use:   "embeddings",
	Short: "Vector embeddings via OpenAI-compat /v1/embeddings",
}

var embeddingsCreateCmd = &cobra.Command{
	Use:   "create <text|->",
	Short: "Compute an embedding for the given text",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input, err := readPrompt(strings.Join(args, " "))
		if err != nil {
			return err
		}
		if input == "" {
			return errors.New("input is empty")
		}
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
		defer cancel()
		req := client.EmbeddingRequest{
			Model: fallback(embModel, "text-embedding-3-small"),
			Input: input,
		}
		out, err := c.Embeddings(ctx, req)
		if err != nil {
			return printError(err)
		}
		if flagJSON || embRaw {
			return ui.PrintJSON(out)
		}
		if len(out.Data) == 0 {
			fmt.Fprintln(os.Stdout, "(no embeddings returned)")
			return nil
		}
		emb := out.Data[0].Embedding
		fmt.Fprintf(os.Stdout, "model:      %s\n", out.Model)
		fmt.Fprintf(os.Stdout, "dimensions: %d\n", len(emb))
		fmt.Fprintf(os.Stdout, "tokens:     %d\n", out.Usage.TotalTokens)
		fmt.Fprint(os.Stdout, "preview:    [")
		for i := 0; i < min(8, len(emb)); i++ {
			if i > 0 {
				fmt.Fprint(os.Stdout, ", ")
			}
			fmt.Fprintf(os.Stdout, "%.4f", emb[i])
		}
		fmt.Fprintln(os.Stdout, ", ...]")
		return nil
	},
}

func init() {
	embeddingsCreateCmd.Flags().StringVar(&embModel, "model", "", "embedding model id (default: text-embedding-3-small)")
	embeddingsCreateCmd.Flags().BoolVar(&embRaw, "raw", false, "always print full JSON")
	embeddingsCmd.AddCommand(embeddingsCreateCmd)
	rootCmd.AddCommand(embeddingsCmd)
}
