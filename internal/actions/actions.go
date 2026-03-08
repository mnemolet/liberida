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
)

type Action struct {
	Type    Type   `json:"type"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
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
		case TypeWrite, TypeRead, TypeDelete, TypeList:
			actions = append(actions, act)
		}
	}
	return actions, nil
}

func (a Action) String() string {
	return fmt.Sprintf("%s: %s", a.Type, a.Path)
}
