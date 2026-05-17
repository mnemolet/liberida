package noninteractive

import (
	"testing"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/provider"
)

func TestDetectInputModeWithArgs(t *testing.T) {
	tests := []struct {
		name       string
		stdinData  string
		args       []string
		wantCtx    string
		wantPrompt string
		wantNonInt bool
	}{
		{
			name:       "empty input",
			stdinData:  "",
			args:       []string{},
			wantCtx:    "",
			wantPrompt: "",
			wantNonInt: false,
		},
		{
			name:       "stdin only",
			stdinData:  "context data",
			args:       []string{},
			wantCtx:    "context data",
			wantPrompt: "",
			wantNonInt: true,
		},
		{
			name:       "args only",
			stdinData:  "",
			args:       []string{"What", "is", "Go?"},
			wantCtx:    "",
			wantPrompt: "What is Go?",
			wantNonInt: true,
		},
		{
			name:       "stdin and args",
			stdinData:  "context info",
			args:       []string{"Explain", "this"},
			wantCtx:    "context info",
			wantPrompt: "Explain this",
			wantNonInt: true,
		},
		{
			name:       "multiple args",
			stdinData:  "",
			args:       []string{"one", "two", "three"},
			wantPrompt: "one two three",
			wantNonInt: true,
		},
		{
			name:       "whitespace preserved in args",
			stdinData:  "",
			args:       []string{"Hello", "World"},
			wantPrompt: "Hello World",
			wantNonInt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, prompt, nonInt := DetectInputModeWithArgs(tt.stdinData, tt.args)

			if ctx != tt.wantCtx {
				t.Errorf("context = %q, want %q", ctx, tt.wantCtx)
			}
			if prompt != tt.wantPrompt {
				t.Errorf("prompt = %q, want %q", prompt, tt.wantPrompt)
			}
			if nonInt != tt.wantNonInt {
				t.Errorf("nonInteractive = %v, want %v", nonInt, tt.wantNonInt)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	cfg := config.DefaultConfig(&config.OSHomeDirProvider{})
	cfg.Model = "test"

	runner := &Runner{
		cfg:   cfg,
		quiet: false,
	}

	tests := []struct {
		name     string
		prompt   string
		context  string
		expected string
	}{
		{
			name:     "empty",
			prompt:   "",
			context:  "",
			expected: "",
		},
		{
			name:     "prompt only",
			prompt:   "Hello",
			context:  "",
			expected: "Question:\nHello",
		},
		{
			name:     "context only",
			prompt:   "",
			context:  "Some context",
			expected: "Context:\nSome context",
		},
		{
			name:     "both",
			prompt:   "Question?",
			context:  "Context data",
			expected: "Context:\nContext data\n\nQuestion:\nQuestion?",
		},
		{
			name:     "multiline context",
			prompt:   "Explain",
			context:  "Line 1\nLine 2\nLine 3",
			expected: "Context:\nLine 1\nLine 2\nLine 3\n\nQuestion:\nExplain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runner.buildPrompt(tt.prompt, tt.context)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNewRunner(t *testing.T) {
	cfg := config.DefaultConfig(&config.OSHomeDirProvider{})
	cfg.Model = "test-model"

	prov := provider.NewMockProvider()

	runner := NewRunner(cfg, prov, true)

	if runner.cfg != cfg {
		t.Error("config not set correctly")
	}
	if runner.prov != prov {
		t.Error("provider not set correctly")
	}
	if !runner.quiet {
		t.Error("quiet should be true")
	}

	runnerQuiet := NewRunner(cfg, prov, false)
	if runnerQuiet.quiet {
		t.Error("quiet should be false")
	}
}

func TestPrepareProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		url      string
		apiKey   string
		wantErr  bool
	}{
		{
			name:     "ollama provider",
			provider: "ollama",
			model:    "llama3.2",
			url:      "http://localhost:11434",
			apiKey:   "",
			wantErr:  false,
		},
		{
			name:     "openrouter provider",
			provider: "openrouter",
			model:    "qwen/qwen3-coder:free",
			url:      "",
			apiKey:   "sk-test-key",
			wantErr:  false,
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			model:    "",
			url:      "",
			apiKey:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Provider:         tt.provider,
				Model:            tt.model,
				OllamaURL:        tt.url,
				OpenRouterAPIKey: tt.apiKey,
			}

			prov, err := PrepareProvider(cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if prov == nil {
				t.Error("provider should not be nil")
			}
		})
	}
}

func TestDetectInputModeWithArgsEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		stdinData  string
		args       []string
		wantNonInt bool
	}{
		{
			name:       "single empty string arg",
			stdinData:  "",
			args:       []string{""},
			wantNonInt: false,
		},
		{
			name:       "valid stdin and empty args",
			stdinData:  "context",
			args:       []string{},
			wantNonInt: true,
		},
		{
			name:       "valid stdin and valid args",
			stdinData:  "context",
			args:       []string{"question"},
			wantNonInt: true,
		},
		{
			name:       "single arg",
			stdinData:  "",
			args:       []string{"question"},
			wantNonInt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, nonInt := DetectInputModeWithArgs(tt.stdinData, tt.args)
			if nonInt != tt.wantNonInt {
				t.Errorf("nonInteractive = %v, want %v", nonInt, tt.wantNonInt)
			}
		})
	}
}