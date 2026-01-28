// Package help styling definitions.
// This file defines lipgloss styles for consistent terminal output.

package help

import "github.com/charmbracelet/lipgloss"

// Styles holds all the lipgloss styles used for help rendering.
type Styles struct {
	// Header is the style for section headers (bold).
	Header lipgloss.Style

	// Subsection is the style for subsection headers like "Global Flags:" (bold).
	Subsection lipgloss.Style

	// Flag is the style for flag names (cyan).
	Flag lipgloss.Style

	// Placeholder is the style for placeholder values (yellow).
	Placeholder lipgloss.Style
}

// DefaultStyles returns the standard styles for help output.
func DefaultStyles() Styles {
	return Styles{
		Header:      lipgloss.NewStyle().Bold(true),
		Subsection:  lipgloss.NewStyle().Bold(true),
		Flag:        lipgloss.NewStyle().Foreground(lipgloss.Color("6")), // Cyan
		Placeholder: lipgloss.NewStyle().Foreground(lipgloss.Color("3")), // Yellow
	}
}
