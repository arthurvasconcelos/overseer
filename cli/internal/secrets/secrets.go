package secrets

import (
	"fmt"
	"os"
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

// RunWithEnv runs the given command with secrets injected from a 1Password
// environment, equivalent to: op run --environment <envID> --no-masking -- cmd
// stdout/stderr/stdin are inherited from the current process.
//
// args must be a real executable — no shell is involved. If you need shell
// features (pipes, functions, expansion), pass the shell explicitly:
//
//	RunWithEnv(envID, "zsh", "-c", "your-command")
func RunWithEnv(envID string, args ...string) error {
	opArgs := []string{"run", "--environment", envID, "--no-masking", "--"}
	opArgs = append(opArgs, args...)
	cmd := exec.Command("op", opArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
