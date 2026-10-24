package symlink

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// Collect maps each file's path relative to its layer dir to the file that
// provides it. Dirs are layered in order, so a file in a later dir replaces the
// same relative path from an earlier one. Missing dirs are skipped.
func Collect(dirs []string) (map[string]string, error) {
	sources := map[string]string{}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			sources[rel] = path
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return sources, nil
}

// MakeAll links every file from the layered dirs (see Collect) into homeDir,
// preserving the relative path: dirs/.config/starship.toml becomes
// homeDir/.config/starship.toml. Existing symlinks into managedRoot are
// overseer's own and get repointed when a different layer wins.
//
// Convention: brain/overseer/dotfiles/ mirrors ~/, and dotfiles.<platform>/
// overlays it for one platform.
func MakeAll(dirs []string, managedRoot, homeDir string, dryRun bool) error {
	sources, err := Collect(dirs)
	if err != nil {
		return err
	}
	rels := make([]string, 0, len(sources))
	for rel := range sources {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		if err := Make(sources[rel], filepath.Join(homeDir, rel), managedRoot, dryRun); err != nil {
			return err
		}
	}
	return nil
}

// Make creates a symlink at target pointing to source, idempotently.
// A symlink that already points into managedRoot is overseer's own and is
// repointed; any other symlink is left alone.
// If dryRun is true no filesystem changes are made, but output is printed.
func Make(source, target, managedRoot string, dryRun bool) error {
	info, err := os.Lstat(target)

	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		current, _ := os.Readlink(target)
		if current == source {
			fmt.Printf("  %s[skip]%s   %s already correct\n", colorGray, colorReset, target)
			return nil
		}
		if !strings.HasPrefix(current, managedRoot+string(filepath.Separator)) {
			fmt.Printf("  %s[warn]%s   %s → symlink points elsewhere (%s), skipping\n", colorYellow, colorReset, target, current)
			return nil
		}
		fmt.Printf("  %s[relink]%s %s → %s\n", colorCyan, colorReset, target, source)
		if dryRun {
			return nil
		}
		if err := os.Remove(target); err != nil {
			return err
		}
		return os.Symlink(source, target)
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
