package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// TextInputModel is a simple text input prompt
type TextInputModel struct {
	textInput textinput.Model
	prompt    string
	value     string
	err       error
	done      bool
}

// NewTextInput creates a new text input prompt
func NewTextInput(prompt, placeholder string) TextInputModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	return TextInputModel{
		textInput: ti,
		prompt:    prompt,
	}
}

func (m TextInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m TextInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.value = m.textInput.Value()
			m.done = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.done = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m TextInputModel) View() string {
	if m.done {
		return ""
	}

	return fmt.Sprintf(
		"%s %s\n\n%s\n\n%s",
		IconDesign,
		SubtitleStyle.Render(m.prompt),
		m.textInput.View(),
		HelpStyle.Render("enter: submit • esc: cancel"),
	)
}

// GetValue returns the entered value
func (m TextInputModel) GetValue() string {
	return m.value
}

// IsDone returns whether the input is complete
func (m TextInputModel) IsDone() bool {
	return m.done
}
