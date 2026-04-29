package executor

import "github.com/mnemolet/liberida/internal/provider"

// GetToolDefinitions returns the JSON schema for your executor methods
func GetToolDefinitions() []provider.Tool {
	return []provider.Tool{
		{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        "write_file",
				Description: "Write content to a file in the workspace",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":    map[string]any{"type": "string", "description": "Relative path to the file"},
						"content": map[string]any{"type": "string", "description": "The text content to write"},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        "run_command",
				Description: "Execute a shell command inside the workspace",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "The command and its arguments as an array",
						},
					},
					"required": []string{"command"},
				},
			},
		},
	}
}
