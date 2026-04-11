// Package notify fires native OS desktop notifications.
// OS detection is performed at runtime; callers are platform-agnostic.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Send fires a native desktop notification. subtitle is optional (pass "" to omit).
//
// OS backends:
//   - macOS  — osascript display notification
//   - Linux  — notify-send (libnotify)
//   - Windows — PowerShell New-BurntToastNotification
func Send(title, message, subtitle string) error {
	switch runtime.GOOS {
	case "darwin":
		return sendMacOS(title, message, subtitle)
	case "linux":
		return sendLinux(title, message)
	case "windows":
		return sendWindows(title, message)
	default:
		return fmt.Errorf("notify: unsupported OS: %s", runtime.GOOS)
	}
}

func sendMacOS(title, message, subtitle string) error {
	var script string
	if subtitle != "" {
		script = fmt.Sprintf(
			`display notification %q with title %q subtitle %q`,
			message, title, subtitle,
		)
	} else {
		script = fmt.Sprintf(
			`display notification %q with title %q`,
			message, title,
		)
	}
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("notify (macOS): %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func sendLinux(title, message string) error {
	out, err := exec.Command("notify-send", title, message).CombinedOutput()
	if err != nil {
		return fmt.Errorf("notify (linux): %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func sendWindows(title, message string) error {
	script := fmt.Sprintf(
		`New-BurntToastNotification -Text %q, %q`,
		title, message,
	)
	out, err := exec.Command("powershell", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("notify (windows): %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
