package secrets

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Binary returns the 1Password CLI executable to invoke. Under WSL the
// desktop app integration only works through the Windows op.exe, which users
// usually reach via a shell alias that exec.Command never sees.
var Binary = sync.OnceValue(func() string {
	if isWSL() {
		if path, err := exec.LookPath("op.exe"); err == nil {
			return path
		}
	}
	return "op"
})

func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// Read retrieves a secret from 1Password using a full op:// reference.
// It shells out to: op read "op://vault/item/field"
func Read(ref string) (string, error) {
	return ReadAs(ref, "")
}

// ReadAs is like Read but targets a specific 1Password account (e.g.
// "my.1password.com"). Use this when the secret lives in a different account
// than the CLI default.
func ReadAs(ref, account string) (string, error) {
	args := []string{"read", ref}
	if account != "" {
		args = append(args, "--account", account)
	}
	out, err := exec.Command(Binary(), args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("op read %s: %s", ref, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("op read %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Get retrieves a secret from 1Password by vault, item, and field name.
func Get(vault, item, field string) (string, error) {
	return Read(fmt.Sprintf("op://%s/%s/%s", vault, item, field))
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
	cmd := exec.Command(Binary(), opArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
