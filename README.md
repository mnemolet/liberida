# Liberida

A local AI agent CLI that runs on your machine using Ollama or OpenRouter. Chat with an AI assistant that can read, write, and execute commands in your workspace.

## Features

- **Local AI Execution**: Runs locally using Ollama or cloud via OpenRouter
- **Tool Calling**: AI can read/write files and run shell commands in your workspace
- **Session Management**: Resume previous conversations or start fresh
- **Workspace Context**: Automatically includes your project files in context (optional)
- **TUI Interface**: Interactive terminal-based chat interface
- **Export**: Export conversations to Markdown or JSON
- **Non-Interactive Mode**: Script-friendly with stdin piping and quiet output

## Requirements

- Go 1.24 or later
- [Ollama](https://github.com/ollama/ollama) (for local models)
- Optional: OpenRouter API key (for cloud models)

## Building

```bash
# Clone the repository
git clone https://github.com/mnemolet/liberida.git
cd liberida

# Build the binary
make build

# Or directly with Go
go build -o bin/liberida ./cmd
```

The binary will be available at `bin/liberida`.

## Testing

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage
```

## Usage

### Basic Chat

```bash
# Start a new chat session (uses Ollama by default with llama3.2)
./bin/liberida

# Start with a specific model
./bin/liberida --new

# Resume an existing session by ID
./bin/liberida -s 5
```

### Non-Interactive Mode (Scripting)

Use Liberida in scripts, pipelines, and automation. If stdin or trailing arguments are provided, the TUI is skipped and the response is printed directly.

```bash
# Trailing arguments as prompt
./bin/liberida "What is 2+2?"

# Pipe context via stdin
echo "function add(a, b) { return a + b; }" | ./bin/liberida "Explain this code"

# Combine stdin context with trailing prompt
cat file.go | ./bin/liberida "What does this do?"

# Quiet mode: only raw response to stdout, no metadata
./bin/liberida -q "What is Go?"

# Pipe context from file
cat error.log | ./bin/liberida -q "What's the issue?"

# Use with Unix tools (grep, jq, etc.)
./bin/liberida -q "list files" | grep ".go"
```

The `--quiet` or `-q` flag suppresses:
- Model information
- Token counts
- Status messages
- All stderr output

Perfect for CI/CD pipelines and shell scripts.

### Attaching Files

Attach files to include their content in the conversation context. Useful for asking the AI to analyze, explain, or work with specific files.

```bash
# Attach a single file
./bin/liberida --attach README.md

# Attach multiple files using repeated flags
./bin/liberida -a error.log -a config.yaml --new

# Attach multiple files using comma-separated values
./bin/liberida -a "main.go,utils.go" --new

# Combine with session resume
./bin/liberida -s 5 --attach debug.log

# Disable automatic workspace context but keep attachments
./bin/liberida --no-context --attach report.md --new
```

Supported file types:
- Text formats: `.txt`, `.log`, `.md`, `.json`, `.yaml`, `.yml`, `.toml`
- Source code: `.go`, `.py`, `.js`, `.ts`, `.java`, `.c`, `.cpp`, `.rs`, `.rb`, `.php`, etc.
- Config files: `.env`, `.ini`, `.cfg`, `.conf`, `.sh`, `.bash`

Constraints:
- Maximum file size: 2MB per file
- Files must exist and be regular files (directories not supported)
- Symlinks are followed automatically

### Configuration

Configuration is stored in `~/.liberida/config.toml`. On first run, a default config is created.

```bash
# View current configuration
./bin/liberida show-config
```

Key configuration options:
- `provider`: "ollama" (default) or "openrouter"
- `model`: LLM model to use
- `ollama_url`: Ollama server URL (default: http://localhost:11434)
- `openrouter_api_key`: Your OpenRouter API key
- `auto_context`: Include workspace files in context
- `auto_title`: Auto-generate session titles

### Session Management

```bash
# List all sessions
./bin/liberida sessions list

# Delete a session
./bin/liberida sessions delete 5
```

### Usage Statistics

```bash
# Show usage for current session
./bin/liberida usage --session 5

# Show total usage across all sessions
./bin/liberida usage total
```

### Exporting Conversations

```bash
# Export current session to stdout (markdown)
./bin/liberida export current

# Export to file
./bin/liberida export current --output my-chat --format md

# Export specific session
./bin/liberida export session 5 --output session-5 --format json

# Export all sessions
./bin/liberida export all --output backups
```

### Using OpenRouter

To use cloud models via OpenRouter:

1. Get an API key from https://openrouter.ai/keys
2. Update config:

```toml
provider = "openrouter"
openrouter_api_key = "sk-or-..."
model = "qwen/qwen3-coder:free"
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

Copyright 2024 Liberida Contributors
