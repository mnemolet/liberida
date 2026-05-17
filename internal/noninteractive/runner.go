package noninteractive

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mnemolet/liberida/internal/attachment"
	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/provider"
	"github.com/mnemolet/liberida/internal/stdinutil"
)

type AttachmentHandler = attachment.Handler

func NewAttachmentHandler() *attachment.Handler {
	return attachment.NewHandler()
}

type Runner struct {
	cfg     *config.Config
	prov    provider.Provider
	quiet   bool
}

func NewRunner(cfg *config.Config, prov provider.Provider, quiet bool) *Runner {
	return &Runner{
		cfg:   cfg,
		prov:  prov,
		quiet: quiet,
	}
}

func (r *Runner) Run(prompt, contextStr string) error {
	if r.quiet {
		return r.runQuiet(prompt, contextStr)
	}
	return r.runVerbose(prompt, contextStr)
}

func (r *Runner) runVerbose(prompt, contextStr string) error {
	fullPrompt := r.buildPrompt(prompt, contextStr)

	msgs := []provider.Message{
		{Role: "system", Content: r.cfg.GetSystemPrompt()},
		{Role: "user", Content: fullPrompt},
	}

	req := provider.Request{
		Model:    r.cfg.Model,
		Messages: msgs,
		Stream:   true,
	}

	chunkChan, _, _, err := r.prov.Stream(context.Background(), req)
	if err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Model: %s\n", r.cfg.Model)

	for chunk := range chunkChan {
		fmt.Print(chunk)
	}

	fmt.Fprintf(os.Stderr, "\n\nDone.\n")
	return nil
}

func (r *Runner) runQuiet(prompt, contextStr string) error {
	fullPrompt := r.buildPrompt(prompt, contextStr)

	msgs := []provider.Message{
		{Role: "system", Content: r.cfg.GetSystemPrompt()},
		{Role: "user", Content: fullPrompt},
	}

	req := provider.Request{
		Model:    r.cfg.Model,
		Messages: msgs,
		Stream:   true,
	}

	chunkChan, _, _, err := r.prov.Stream(context.Background(), req)
	if err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	for chunk := range chunkChan {
		fmt.Print(chunk)
	}

	return nil
}

func (r *Runner) buildPrompt(prompt, contextStr string) string {
	var parts []string

	if contextStr != "" {
		parts = append(parts, "Context:\n"+contextStr)
	}
	if prompt != "" {
		parts = append(parts, "Question:\n"+prompt)
	}

	return strings.Join(parts, "\n\n")
}

func DetectInputMode() (stdinData string, prompt string, isNonInteractive bool) {
	if stdinutil.IsPiped() {
		data, err := stdinutil.ReadAllTrimmed()
		if err == nil && data != "" {
			stdinData = data
		}
	}
	return stdinData, "", stdinData != ""
}

func DetectInputModeWithArgs(stdinData string, args []string) (contextStr string, prompt string, isNonInteractive bool) {
	contextStr = stdinData

	if len(args) > 0 {
		prompt = strings.Join(args, " ")
	}

	return contextStr, prompt, contextStr != "" || prompt != ""
}

func PrepareProvider(cfg *config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "ollama":
		return provider.NewOllamaProvider(cfg.OllamaURL, cfg.Model), nil
	case "openrouter":
		return provider.NewProvider("openrouter", "", cfg.Model, cfg.OpenRouterAPIKey)
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func QuietReader(ctx context.Context, prov provider.Provider, model string, systemPrompt, userPrompt string, output io.Writer) error {
	msgs := []provider.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	req := provider.Request{
		Model:    model,
		Messages: msgs,
		Stream:   true,
	}

	chunkChan, _, _, err := prov.Stream(ctx, req)
	if err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	for chunk := range chunkChan {
		io.WriteString(output, chunk)
	}

	return nil
}