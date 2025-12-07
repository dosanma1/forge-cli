package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// MultiSelectModel is a multi-selection list with checkboxes
type MultiSelectModel struct {
	prompt   string
	choices  []string
	cursor   int
	selected map[int]bool
	done     bool
}

// NewMultiSelect creates a new multi-selection prompt
func NewMultiSelect(prompt string, choices []string, defaultSelected []int) MultiSelectModel {
	selected := make(map[int]bool)
	for _, idx := range defaultSelected {
		if idx >= 0 && idx < len(choices) {
			selected[idx] = true
		}
	}

	return MultiSelectModel{
		prompt:   prompt,
		choices:  choices,
		cursor:   0,
		selected: selected,
	}
}

func (m MultiSelectModel) Init() tea.Cmd {
	return nil
}

func (m MultiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case " ", "x":
			// Toggle selection
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "enter":
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m MultiSelectModel) View() string {
	if m.done {
		return ""
	}

	s := fmt.Sprintf("%s %s\n\n", IconDesign, SubtitleStyle.Render(m.prompt))

	for i, choice := range m.choices {
		cursor := " "
		checkbox := "[ ]"
		style := UnselectedStyle

		if i == m.cursor {
			cursor = ">"
			style = SelectedStyle
		}

		if m.selected[i] {
			checkbox = "[✓]"
		}

		s += fmt.Sprintf("  %s %s %s\n", cursor, checkbox, style.Render(choice))
	}

	s += fmt.Sprintf("\n%s", HelpStyle.Render("↑/↓: navigate • space: toggle • enter: confirm • esc: cancel"))

	return s
}

// GetSelected returns the indices of selected choices
func (m MultiSelectModel) GetSelected() []int {
	var result []int
	for i := 0; i < len(m.choices); i++ {
		if m.selected[i] {
			result = append(result, i)
		}
	}
	return result
}

// GetSelectedValues returns the values of selected choices
func (m MultiSelectModel) GetSelectedValues() []string {
	var result []string
	for i := 0; i < len(m.choices); i++ {
		if m.selected[i] {
			result = append(result, m.choices[i])
		}
	}
	return result
}

// IsDone returns whether selection is complete
func (m MultiSelectModel) IsDone() bool {
	return m.done
}
