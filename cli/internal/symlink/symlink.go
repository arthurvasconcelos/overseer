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

// LinkAll links every source (see Collect) into homeDir at its relative path:
// .config/starship.toml becomes homeDir/.config/starship.toml. Existing
// symlinks into managedRoot are overseer's own and get repointed when a
// different layer wins.
//
// Convention: brain/overseer/dotfiles/ mirrors ~/, and dotfiles.<platform>/
// overlays it for one platform.
func LinkAll(sources map[string]string, managedRoot, homeDir string, dryRun bool) error {
	for _, rel := range sortedKeys(sources) {
		if err := Make(sources[rel], filepath.Join(homeDir, rel), managedRoot, dryRun); err != nil {
			return err
		}
	}
	return nil
}

// Conflicts returns the relative paths whose target in homeDir is a real file
// or directory, which linking would move to the backup directory.
func Conflicts(sources map[string]string, homeDir string) []string {
	var conflicts []string
	for _, rel := range sortedKeys(sources) {
		info, err := os.Lstat(filepath.Join(homeDir, rel))
		if err == nil && info.Mode()&os.ModeSymlink == 0 {
			conflicts = append(conflicts, rel)
		}
	}
	return conflicts
}

// Prune removes overseer's links for files that exist in the brain but no
// longer apply here: known holds every relative path in any layer, for any
// platform, and active the ones linked on this platform. Only symlinks into
// managedRoot are removed. When the link had replaced a file, the latest
// backup of it is moved back into place.
func Prune(known, active map[string]string, managedRoot, homeDir string, dryRun bool) error {
	for _, rel := range sortedKeys(known) {
		if _, ok := active[rel]; ok {
			continue
		}
		target := filepath.Join(homeDir, rel)
		info, err := os.Lstat(target)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		current, _ := os.Readlink(target)
		if !strings.HasPrefix(current, managedRoot+string(filepath.Separator)) {
			continue
		}
		fmt.Printf("  %s[unlink]%s %s (%s doesn't apply on this platform)\n", colorYellow, colorReset, target, rel)
		if !dryRun {
			if err := os.Remove(target); err != nil {
				return err
			}
		}
		backup := latestBackup(homeDir, rel)
		if backup == "" {
			continue
		}
		fmt.Printf("  %s[restore]%s %s ← %s\n", colorGreen, colorReset, target, backup)
		if !dryRun {
			if err := os.Rename(backup, target); err != nil {
				return err
			}
		}
	}
	return nil
}

// backupName is the path a target is stored under in the backup directory:
// its path relative to the home folder, so files that share a name don't clash.
func backupName(target string) string {
	home, _ := os.UserHomeDir()
	if rel, err := filepath.Rel(home, target); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return filepath.Base(target)
}

// latestBackup returns the most recent backup of rel under
// homeDir/.overseer-backups, or "" when there is none.
func latestBackup(homeDir, rel string) string {
	backups, _ := filepath.Glob(filepath.Join(homeDir, ".overseer-backups", "*", rel))
	if len(backups) == 0 {
		return ""
	}
	sort.Strings(backups)
	return backups[len(backups)-1]
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
		dest := filepath.Join(getBackupDir(), backupName(target))
		fmt.Printf("  %s[backup]%s %s → %s\n", colorCyan, colorReset, target, dest)
		if !dryRun {
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
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
