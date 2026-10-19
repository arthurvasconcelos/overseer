package wsl

import (
	"os"
	"strings"
	"sync"
)

// Detected reports whether overseer is running inside Windows Subsystem for Linux.
var Detected = sync.OnceValue(func() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(data)), "microsoft")
})
