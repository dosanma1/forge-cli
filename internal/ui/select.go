package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// SelectModel is a single-selection list
type SelectModel struct {
	prompt   string
	choices  []string
	cursor   int
	selected int
	done     bool
}

// NewSelect creates a new selection prompt
func NewSelect(prompt string, choices []string) SelectModel {
	return SelectModel{
		prompt:   prompt,
		choices:  choices,
		cursor:   0,
		selected: -1,
	}
}

func (m SelectModel) Init() tea.Cmd {
	return nil
}

func (m SelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.cursor
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m SelectModel) View() string {
	if m.done {
		return ""
	}

	s := fmt.Sprintf("%s %s\n\n", IconDesign, SubtitleStyle.Render(m.prompt))

	for i, choice := range m.choices {
		cursor := " "
		style := UnselectedStyle
		if i == m.cursor {
			cursor = ">"
			style = SelectedStyle
		}

		s += fmt.Sprintf("  %s %s\n", cursor, style.Render(choice))
	}

	s += fmt.Sprintf("\n%s", HelpStyle.Render("↑/↓: navigate • enter: select • esc: cancel"))

	return s
}

// GetSelected returns the selected choice index
func (m SelectModel) GetSelected() int {
	return m.selected
}

// GetSelectedValue returns the selected choice value
func (m SelectModel) GetSelectedValue() string {
	if m.selected >= 0 && m.selected < len(m.choices) {
		return m.choices[m.selected]
	}
	return ""
}

// IsDone returns whether selection is complete
func (m SelectModel) IsDone() bool {
	return m.done
}
