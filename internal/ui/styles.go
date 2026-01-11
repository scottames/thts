// Package ui provides consistent terminal output styling for the thts CLI.
package ui

import "github.com/charmbracelet/lipgloss"

// Color definitions using ANSI 256-color codes for terminal compatibility.
var (
	ColorSuccess = lipgloss.Color("2") // green
	ColorInfo    = lipgloss.Color("4") // blue
	ColorWarning = lipgloss.Color("3") // yellow
	ColorError   = lipgloss.Color("1") // red
	ColorMuted   = lipgloss.Color("8") // gray
	ColorAccent  = lipgloss.Color("6") // cyan
)

// Base styles for text rendering.
var (
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleInfo    = lipgloss.NewStyle().Foreground(ColorInfo)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError)
	StyleMuted   = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleAccent  = lipgloss.NewStyle().Foreground(ColorAccent)
)

// Unicode symbols for message prefixes.
const (
	SymbolSuccess = "✓" // U+2713 CHECK MARK
	SymbolInfo    = "ℹ" // U+2139 INFORMATION SOURCE
	SymbolWarning = "⚠" // U+26A0 WARNING SIGN
	SymbolError   = "✗" // U+2717 BALLOT X
	SymbolBullet  = "•" // U+2022 BULLET
)
