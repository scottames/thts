package ui

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Table styles.
var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorAccent)

	TableCellStyle = lipgloss.NewStyle()

	TableBorderStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)
)

// NewTable creates a styled table with headers.
func NewTable(headers ...string) *table.Table {
	t := table.New().
		Headers(headers...).
		BorderStyle(TableBorderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return TableHeaderStyle
			}
			return TableCellStyle
		})

	return t
}

// KeyValueTable creates a simple key-value display table.
// Each row should be a [key, value] pair.
func KeyValueTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			if col == 0 {
				return StyleMuted
			}
			return StyleAccent
		})

	for _, row := range rows {
		t.Row(row...)
	}

	return t.String()
}
