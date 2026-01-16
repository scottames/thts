package ui

import (
	"errors"
	"os"

	"github.com/charmbracelet/huh"
)

// ErrNotTerminal is returned when an operation requires a terminal but stdin is not a TTY.
var ErrNotTerminal = errors.New("not a terminal")

// ErrEmptyInput is returned when the user provides empty input where a value is required.
var ErrEmptyInput = errors.New("input cannot be empty")

// IsTerminal checks if stdin is a terminal.
func IsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// PromptForInput prompts the user for text input with the given title.
// Returns ErrNotTerminal if stdin is not a TTY.
// Returns ErrEmptyInput if the user enters an empty value.
func PromptForInput(title, placeholder string) (string, error) {
	if !IsTerminal() {
		return "", ErrNotTerminal
	}

	var value string
	err := huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(&value).
		Run()
	if err != nil {
		return "", err
	}

	if value == "" {
		return "", ErrEmptyInput
	}

	return value, nil
}
