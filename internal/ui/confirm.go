package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// ConfirmModel is a yes/no confirmation prompt
type ConfirmModel struct {
	prompt       string
	defaultValue bool
	selected     bool
	confirmed    bool
	cancelled    bool
}

// NewConfirm creates a new confirmation prompt
func NewConfirm(prompt string, defaultYes bool) ConfirmModel {
	return ConfirmModel{
		prompt:       prompt,
		defaultValue: defaultYes,
		selected:     defaultYes,
	}
}

func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.selected = true
			m.confirmed = true
			return m, tea.Quit
		case "n", "N":
			m.selected = false
			m.confirmed = true
			return m, tea.Quit
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		case "left", "right", "tab":
			m.selected = !m.selected
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m ConfirmModel) View() string {
	if m.confirmed || m.cancelled {
		return ""
	}

	yesStyle := UnselectedStyle
	noStyle := UnselectedStyle

	if m.selected {
		yesStyle = SelectedStyle
	} else {
		noStyle = SelectedStyle
	}

	return fmt.Sprintf(
		"%s %s\n\n  %s Yes    %s No\n\n%s",
		IconDesign,
		SubtitleStyle.Render(m.prompt),
		yesStyle.Render(">"),
		noStyle.Render(">"),
		HelpStyle.Render("←/→: toggle • enter: confirm • y/n: quick select • esc: cancel"),
	)
}

// IsConfirmed returns whether the user confirmed (and said yes)
func (m ConfirmModel) IsConfirmed() bool {
	return m.confirmed && m.selected
}

// IsCancelled returns whether the user cancelled
func (m ConfirmModel) IsCancelled() bool {
	return m.cancelled
}
