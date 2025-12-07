package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Forge brand colors and styles
var (
	// Colors matching Forge emoji branding
	ColorBlue   = lipgloss.Color("63")  // 🔧 Tools/Technical
	ColorPurple = lipgloss.Color("141") // 🎨 Design/Creative
	ColorGreen  = lipgloss.Color("42")  // ✅ Success
	ColorYellow = lipgloss.Color("220") // ⚠️  Warning
	ColorRed    = lipgloss.Color("196") // ❌ Error
	ColorGray   = lipgloss.Color("240") // Subtle text

	// Text styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPurple).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorBlue).
			MarginBottom(1)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorGreen).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorYellow)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorGray)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(ColorPurple).
			Bold(true).
			PaddingLeft(2)

	UnselectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			PaddingLeft(2)

	// Emoji icons
	IconTool    = "🔧"
	IconDesign  = "🎨"
	IconSuccess = "✅"
	IconWarning = "⚠️ "
	IconError   = "❌"
	IconRocket  = "🚀"
	IconPackage = "📦"
)
