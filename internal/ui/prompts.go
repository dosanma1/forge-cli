package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// AskText prompts for text input
func AskText(prompt, placeholder string) (string, error) {
	m := NewTextInput(prompt, placeholder)
	p := tea.NewProgram(m)
	
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	result := finalModel.(TextInputModel)
	if !result.IsDone() || result.GetValue() == "" {
		return "", fmt.Errorf("cancelled")
	}

	return result.GetValue(), nil
}

// AskConfirm prompts for yes/no confirmation
func AskConfirm(prompt string, defaultYes bool) (bool, error) {
	m := NewConfirm(prompt, defaultYes)
	p := tea.NewProgram(m)
	
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	result := finalModel.(ConfirmModel)
	if result.IsCancelled() {
		return false, fmt.Errorf("cancelled")
	}

	return result.IsConfirmed(), nil
}

// AskSelect prompts for single selection
func AskSelect(prompt string, choices []string) (int, string, error) {
	m := NewSelect(prompt, choices)
	p := tea.NewProgram(m)
	
	finalModel, err := p.Run()
	if err != nil {
		return -1, "", err
	}

	result := finalModel.(SelectModel)
	if !result.IsDone() || result.GetSelected() < 0 {
		return -1, "", fmt.Errorf("cancelled")
	}

	return result.GetSelected(), result.GetSelectedValue(), nil
}

// AskMultiSelect prompts for multiple selections
func AskMultiSelect(prompt string, choices []string, defaultSelected []int) ([]int, []string, error) {
	m := NewMultiSelect(prompt, choices, defaultSelected)
	p := tea.NewProgram(m)
	
	finalModel, err := p.Run()
	if err != nil {
		return nil, nil, err
	}

	result := finalModel.(MultiSelectModel)
	if !result.IsDone() {
		return nil, nil, fmt.Errorf("cancelled")
	}

	return result.GetSelected(), result.GetSelectedValues(), nil
}
