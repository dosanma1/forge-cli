package ui

import (
	"github.com/manifoldco/promptui"
)

// Prompter wraps promptui for consistent UI interactions
type Prompter struct{}

// NewPrompter creates a new Prompter instance
func NewPrompter() (*Prompter, error) {
	return &Prompter{}, nil
}

// AskText prompts for text input
func (p *Prompter) AskText(label string, defaultValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}

	return prompt.Run()
}

// AskConfirm prompts for yes/no confirmation
func (p *Prompter) AskConfirm(label string, defaultValue bool) (bool, error) {
	defaultText := "N"
	if defaultValue {
		defaultText = "Y"
	}

	prompt := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
		Default:   defaultText,
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return result == "y" || result == "Y" || result == "", nil
}

// AskSelect prompts for selection from a list
func (p *Prompter) AskSelect(label string, items []string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}

	_, result, err := prompt.Run()
	return result, err
}

// AskMultiSelect prompts for multiple selections (not implemented in promptui, returns single select)
func (p *Prompter) AskMultiSelect(label string, items []string) ([]string, error) {
	// promptui doesn't support multi-select, so we'll do single select for now
	result, err := p.AskSelect(label, items)
	if err != nil {
		return nil, err
	}
	return []string{result}, nil
}

// Package-level convenience functions
var defaultPrompter = &Prompter{}

// AskText prompts for text input (convenience function)
func AskText(label string, defaultValue string) (string, error) {
	return defaultPrompter.AskText(label, defaultValue)
}

// AskConfirm prompts for yes/no confirmation (convenience function)
func AskConfirm(label string, defaultValue bool) (bool, error) {
	return defaultPrompter.AskConfirm(label, defaultValue)
}

// AskSelect prompts for selection from a list (convenience function)
func AskSelect(label string, items []string) (int, string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}

	return prompt.Run()
}
