package cmd

import (
	"fmt"
	"os/exec"
	"strings"
)

// copyToClipboard writes text to the macOS clipboard via pbcopy.
func copyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy: %w", err)
	}
	return nil
}
