package secrets

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/arthurvasconcelos/overseer/internal/wsl"
)

// Binary returns the 1Password CLI executable to invoke. Under WSL the
// desktop app integration only works through the Windows op.exe, which users
// usually reach via a shell alias that exec.Command never sees.
var Binary = sync.OnceValue(func() string {
	if wsl.Detected() {
		if path, err := exec.LookPath("op.exe"); err == nil {
			return path
		}
	}
	return "op"
})

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
			return "", fmt.Errorf("op read %s: %s", ref, explainStderr(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("op read %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// explainStderr replaces op's long account-setup help with a short hint when
// the CLI cannot reach the desktop app, which is almost always the real cause.
func explainStderr(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if !strings.Contains(stderr, "No accounts configured") {
		return stderr
	}
	hint := "1Password CLI cannot see any accounts. Make sure the 1Password app is running " +
		"with Settings → Developer → \"Integrate with 1Password CLI\" turned on, then check with `op account list`."
	if runtime.GOOS == "darwin" {
		hint += " If it is still empty, run `op account list --debug`: \"operation not permitted\" on settings.json " +
			"means macOS is blocking your terminal from 1Password's app data (System Settings → Privacy & Security)."
	}
	return hint
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
