package tui

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// SelectItem is a single option shown in the selector.
type SelectItem struct {
	Title    string
	Subtitle string // optional, shown beside the title
}

// Select shows an interactive arrow-key selector and returns the chosen index.
// Returns -1 if the user cancelled.
func Select(title string, items []SelectItem) (int, error) {
	opts := make([]huh.Option[int], len(items))
	for i, item := range items {
		label := item.Title
		if item.Subtitle != "" {
			label += "  " + item.Subtitle
		}
		opts[i] = huh.NewOption(label, i)
	}

	choice := -1
	err := huh.NewSelect[int]().
		Title(title).
		Options(opts...).
		Value(&choice).
		Run()

	if errors.Is(err, huh.ErrUserAborted) {
		return -1, nil
	}
	if err != nil {
		return -1, fmt.Errorf("tui: %w", err)
	}
	return choice, nil
}

// Confirm shows an interactive yes/no prompt and returns true if the user confirmed.
// Returns false (no error) if the user cancels.
func Confirm(title string) (bool, error) {
	var confirmed bool
	err := huh.NewConfirm().
		Title(title).
		Value(&confirmed).
		Run()

	if errors.Is(err, huh.ErrUserAborted) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("tui: %w", err)
	}
	return confirmed, nil
}
