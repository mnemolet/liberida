package noninteractive

import (
	"testing"

	"github.com/mnemolet/liberida/internal/config"
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