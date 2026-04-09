package tui

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// Prompt displays an interactive single-line text input and returns the entered value.
// If the user submits an empty value and a defaultVal is provided, the defaultVal is returned.
// Returns an error if the user cancels.
func Prompt(label, defaultVal, placeholder string) (string, error) {
	val := defaultVal
	err := huh.NewInput().
		Title(label).
		Value(&val).
		Placeholder(placeholder).
		Run()

	if errors.Is(err, huh.ErrUserAborted) {
		return "", fmt.Errorf("cancelled")
	}
	if err != nil {
		return "", fmt.Errorf("tui: %w", err)
	}
	if val == "" {
		val = placeholder
	}
	return val, nil
}
