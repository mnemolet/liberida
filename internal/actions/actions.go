package actions

import (
	"encoding/json"
	"fmt"
	"regexp"
)

type Type string

const (
	TypeWrite  Type = "write"
	TypeRead   Type = "read"
	TypeDelete Type = "delete"
	TypeList   Type = "list"
	TypeExec   Type = "exec"
)

type Action struct {
	Type    Type     `json:"type"`
	Path    string   `json:"path,omitempty"`
	Content string   `json:"content,omitempty"`
	Command []string `json:"command,omitempty"`
}

// Parse extracts actions from text by looking for JSON objects.
// Returns slice of valid actions.
func Parse(text string) ([]Action, error) {
	var actions []Action
	re := regexp.MustCompile(`\{[^{}]*\}`)
	matches := re.FindAllString(text, -1)
	for _, match := range matches {
		var act Action
		if err := json.Unmarshal([]byte(match), &act); err != nil {
			continue
		}
		switch act.Type {
		case TypeWrite, TypeRead, TypeDelete, TypeList, TypeExec:
			actions = append(actions, act)
		}
	}
	return actions, nil
}

func (a Action) String() string {
	switch a.Type {
	case TypeWrite:
		return fmt.Sprintf("write: %s", a.Path)
	case TypeRead:
		return fmt.Sprintf("read: %s", a.Path)
	case TypeDelete:
		return fmt.Sprintf("delete: %s", a.Path)
	case TypeList:
		return "list files"
	case TypeExec:
		return fmt.Sprintf("exec: %v", a.Command)
	default:
		return "unknown action"
	}
}
