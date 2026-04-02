package symlink

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// backupDir is shared across all Make calls in a single run so everything
// lands in the same timestamped directory.
var backupDir string

func getBackupDir() string {
	if backupDir == "" {
		home, _ := os.UserHomeDir()
		backupDir = filepath.Join(home, ".overseer-backups", time.Now().Format("20060102_150405"))
	}
	return backupDir
}

// Make creates a symlink at target pointing to source, idempotently.
// If dryRun is true no filesystem changes are made, but output is printed.
func Make(source, target string, dryRun bool) error {
	info, err := os.Lstat(target)

	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		current, _ := os.Readlink(target)
		if current == source {
			fmt.Printf("  %s[skip]%s   %s already correct\n", colorGray, colorReset, target)
			return nil
		}
		fmt.Printf("  %s[warn]%s   %s → symlink points elsewhere (%s), skipping\n", colorYellow, colorReset, target, current)
		return nil
	}

	if err == nil {
		dir := getBackupDir()
		dest := filepath.Join(dir, filepath.Base(target))
		fmt.Printf("  %s[backup]%s %s → %s\n", colorCyan, colorReset, target, dest)
		if !dryRun {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			if err := os.Rename(target, dest); err != nil {
				return err
			}
		}
	}

	fmt.Printf("  %s[link]%s   %s → %s\n", colorGreen, colorReset, target, source)
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.Symlink(source, target); err != nil {
			return err
		}
	}

	return nil
}
