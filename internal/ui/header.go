package ui

import "charm.land/lipgloss/v2"

// HeaderStyle defines the lipgloss style for header boxes.
var HeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorInfo).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(ColorMuted).
	Padding(0, 1)

// SubHeaderStyle defines the style for subsection headers.
var SubHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorWarning)

// Header renders a title in a bordered box.
func Header(title string) string {
	return HeaderStyle.Render(title)
}

// SubHeader renders a subsection header (bold, no border).
func SubHeader(title string) string {
	return SubHeaderStyle.Render(title)
}
