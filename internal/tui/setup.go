package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	choices   []string
	cursor    int
	question  string
	step      int
	completed bool
}

func InitialModel() Model {
	return Model{
		step:      0,
		cursor:    0,
		completed: false,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	s := "\n LiberIda Setup Wizard\n\n"

	if m.completed {
		s += "Setup complete! Press q to exit.\n"
	} else {
		s += "Press q to quit\n"
	}
	return s
}
