package secrets

import (
	"fmt"
	"os/exec"
	"strings"
)

// Get retrieves a secret from 1Password using the op CLI.
// It shells out to: op read "op://vault/item/field"
func Get(vault, item, field string) (string, error) {
	ref := fmt.Sprintf("op://%s/%s/%s", vault, item, field)
	out, err := exec.Command("op", "read", ref).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("op read %s: %s", ref, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("op read %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}
