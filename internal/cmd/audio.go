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

var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Speech (TTS) and transcription (STT) commands",
}

// ---------- speak (TTS) ----------

var (
	ttsModel string
	ttsVoice string
	ttsSpeed float64
	ttsOut   string
	ttsFmt   string
)

var audioSpeakCmd = &cobra.Command{
	Use:   "speak <text>",
	Short: "Synthesise speech (TTS) and write it to disk",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		req := client.SpeechRequest{
			Model:          fallback(ttsModel, "tts-1"),
			Input:          strings.Join(args, " "),
			Voice:          ttsVoice,
			ResponseFormat: ttsFmt,
			Speed:          ttsSpeed,
		}
		body, ctype, err := c.Speak(ctx, req)
		if err != nil {
			return printError(err)
		}
		path := ttsOut
		if path == "" {
			path = "tts" + extensionForContentType(ctype)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Saved: %s (%s, %d bytes)\n", path, ctype, len(body))
		return nil
	},
}

func extensionForContentType(ct string) string {
	switch {
	case strings.Contains(ct, "mpeg"), strings.Contains(ct, "mp3"):
		return ".mp3"
	case strings.Contains(ct, "wav"):
		return ".wav"
	case strings.Contains(ct, "ogg"):
		return ".ogg"
	case strings.Contains(ct, "opus"):
		return ".opus"
	case strings.Contains(ct, "flac"):
		return ".flac"
	default:
		return ".audio"
	}
}

// ---------- transcribe (STT) ----------

var (
	sttModel    string
	sttLanguage string
	sttFmt      string
	sttPrompt   string
)

var audioTranscribeCmd = &cobra.Command{
	Use:   "transcribe <audio-file>",
	Short: "Transcribe an audio file (STT)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		if _, err := os.Stat(args[0]); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("file not found: %s", args[0])
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		out, err := c.Transcribe(ctx, args[0], client.TranscribeOptions{
			Model:          fallback(sttModel, "whisper-large-v3"),
			Language:       sttLanguage,
			ResponseFormat: sttFmt,
			Prompt:         sttPrompt,
		})
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		fmt.Fprintln(os.Stdout, out.Text)
		return nil
	},
}

// ---------- voices ----------

var audioVoicesCmd = &cobra.Command{
	Use:   "voices",
	Short: "List available TTS voices",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := resolveClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		out, err := c.ListVoices(ctx)
		if err != nil {
			return printError(err)
		}
		if flagJSON {
			return ui.PrintJSON(out)
		}
		t := ui.NewTable(os.Stdout)
		t.Headers("ID", "NAME", "LANG", "PROVIDER", "GENDER")
		for _, v := range out.Data {
			t.Row(v.ID, v.Name, v.Language, v.Provider, v.Gender)
		}
		return t.Flush()
	},
}

func init() {
	audioSpeakCmd.Flags().StringVar(&ttsModel, "model", "", "TTS model id (default: tts-1)")
	audioSpeakCmd.Flags().StringVar(&ttsVoice, "voice", "", "voice id (see `parel audio voices`)")
	audioSpeakCmd.Flags().Float64Var(&ttsSpeed, "speed", 0, "speech speed (model-dependent)")
	audioSpeakCmd.Flags().StringVar(&ttsFmt, "format", "", "audio format (mp3 / wav / opus / flac)")
	audioSpeakCmd.Flags().StringVarP(&ttsOut, "out", "o", "", "output file (default: tts.<ext>)")

	audioTranscribeCmd.Flags().StringVar(&sttModel, "model", "", "STT model id (default: whisper-large-v3)")
	audioTranscribeCmd.Flags().StringVar(&sttLanguage, "language", "", "ISO-639-1 language code")
	audioTranscribeCmd.Flags().StringVar(&sttFmt, "response-format", "", "json / text / verbose_json / srt / vtt")
	audioTranscribeCmd.Flags().StringVar(&sttPrompt, "prompt", "", "biasing prompt")

	audioCmd.AddCommand(audioSpeakCmd, audioTranscribeCmd, audioVoicesCmd)
	rootCmd.AddCommand(audioCmd)
}
