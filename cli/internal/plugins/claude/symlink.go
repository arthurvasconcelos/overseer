package claude

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arthurvasconcelos/overseer/internal/tui"
)

// linkKind describes how a target is symlinked into ~/.claude/.
type linkKind int

const (
	// linkFile creates a single symlink for the file at brainPath → localPath.
	linkFile linkKind = iota
	// linkWholeDir creates a single symlink for the entire directory.
	linkWholeDir
	// linkChildren walks brainPath and creates individual symlinks for each
	// immediate child (file or dir) into the localPath directory.
	linkChildren
)

// managedTarget describes one item the Claude plugin tracks.
type managedTarget struct {
	name      string   // display label, e.g. "settings.json"
	brainRel  string   // path relative to <brain>/claude/, e.g. "skills"
	localPath string   // absolute path in ~/.claude/, e.g. "~/.claude/skills"
	kind      linkKind
}

// targetAction describes what setup will do for a given target.
type targetAction int

const (
	actionSkip     targetAction = iota // already correctly linked, nothing to do
	actionLinkOnly                     // brain file exists, local missing → create symlink
	actionAdopt                        // local file exists (not symlink) → move to brain, symlink
	actionMigrate                      // local symlinks to old brain path → move brain file, update symlink
	actionConflict                     // both brain and local exist with different content → user must resolve
)

// targetScan holds the result of scanning one managed target.
type targetScan struct {
	target managedTarget
	action targetAction
	detail string // human-readable explanation shown in the wizard
}

// wellKnownTargets returns the fixed set of targets the Claude plugin manages,
// with localPath resolved to absolute paths.
func wellKnownTargets(claudeDir string) []managedTarget {
	home, _ := os.UserHomeDir()
	dotClaude := filepath.Join(home, ".claude")
	return []managedTarget{
		{name: "CLAUDE.md", brainRel: "CLAUDE.md", localPath: filepath.Join(dotClaude, "CLAUDE.md"), kind: linkFile},
		{name: "settings.json", brainRel: "settings.json", localPath: filepath.Join(dotClaude, "settings.json"), kind: linkFile},
		{name: "plans/", brainRel: "plans", localPath: filepath.Join(dotClaude, "plans"), kind: linkWholeDir},
		{name: "memory/", brainRel: "memory", localPath: filepath.Join(dotClaude, "memory"), kind: linkWholeDir},
		{name: "hooks/", brainRel: "hooks", localPath: filepath.Join(dotClaude, "hooks"), kind: linkChildren},
		{name: "skills/", brainRel: "skills", localPath: filepath.Join(dotClaude, "skills"), kind: linkChildren},
	}
}

// scanTarget examines the state of a single managed target and determines
// the appropriate action.
func scanTarget(t managedTarget, brainClaudeDir, oldBrainDir string) targetScan {
	brainPath := filepath.Join(brainClaudeDir, t.brainRel)

	// For linkChildren targets, we scan each child rather than the parent.
	// The parent-level scan just checks if the brain dir exists and reports children.
	if t.kind == linkChildren {
		return scanChildrenTarget(t, brainPath, oldBrainDir)
	}

	brainExists := pathExists(brainPath)

	localInfo, localErr := os.Lstat(t.localPath)
	localExists := localErr == nil
	localIsSymlink := localExists && localInfo.Mode()&os.ModeSymlink != 0

	if localIsSymlink {
		current, _ := os.Readlink(t.localPath)
		if current == brainPath {
			return targetScan{target: t, action: actionSkip, detail: "already symlinked"}
		}
		// Check if it points to old brain root path
		oldPath := filepath.Join(oldBrainDir, t.brainRel)
		if current == oldPath {
			if brainExists {
				return targetScan{target: t, action: actionConflict,
					detail: fmt.Sprintf("old symlink → %s; brain/claude/%s also exists — manual resolution needed", current, t.brainRel)}
			}
			return targetScan{target: t, action: actionMigrate,
				detail: fmt.Sprintf("old symlink → %s → will move to brain/claude/ and relink", current)}
		}
		// Points somewhere else entirely
		return targetScan{target: t, action: actionConflict,
			detail: fmt.Sprintf("symlink → %s (unrecognised target — resolve manually)", current)}
	}

	if localExists && brainExists {
		return targetScan{target: t, action: actionConflict,
			detail: "exists both locally and in brain/claude/ — resolve manually"}
	}

	if localExists && !brainExists {
		return targetScan{target: t, action: actionAdopt,
			detail: "local file will be moved to brain/claude/ and symlinked"}
	}

	if !localExists && brainExists {
		return targetScan{target: t, action: actionLinkOnly,
			detail: "brain file exists — will create symlink"}
	}

	// Neither exists.
	return targetScan{target: t, action: actionSkip, detail: "not present anywhere (skip)"}
}

// scanChildrenTarget handles linkChildren targets: individual children of the
// brain dir are symlinked into the local dir one by one.
func scanChildrenTarget(t managedTarget, brainPath, oldBrainDir string) targetScan {
	brainExists := pathExists(brainPath)
	if !brainExists {
		return targetScan{target: t, action: actionSkip, detail: "no brain/claude/" + t.brainRel + "/ directory (skip)"}
	}

	entries, err := os.ReadDir(brainPath)
	if err != nil || len(entries) == 0 {
		return targetScan{target: t, action: actionSkip, detail: "brain/claude/" + t.brainRel + "/ is empty (skip)"}
	}

	return targetScan{
		target: t,
		action: actionLinkOnly,
		detail: fmt.Sprintf("%d item(s) in brain/claude/%s/ will be individually symlinked", len(entries), t.brainRel),
	}
}

// applyTarget executes the action for a single target. Returns true if a
// follow-up action was taken, false if skipped.
func applyTarget(scan targetScan, brainClaudeDir, oldBrainDir string, dryRun bool) error {
	t := scan.target
	brainPath := filepath.Join(brainClaudeDir, t.brainRel)

	switch scan.action {
	case actionSkip:
		fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("skip  "), tui.StyleDim.Render(t.name))
		return nil

	case actionLinkOnly:
		if t.kind == linkChildren {
			return applyChildren(t, brainPath, dryRun)
		}
		return makeLink(brainPath, t.localPath, dryRun, t.name)

	case actionAdopt:
		return applyAdopt(t, brainPath, dryRun)

	case actionMigrate:
		return applyMigrate(t, brainPath, oldBrainDir, dryRun)

	case actionConflict:
		fmt.Printf("  %s  %s  %s\n",
			tui.StyleWarn.Render("warn  "),
			tui.StyleNormal.Render(t.name),
			tui.StyleDim.Render(scan.detail),
		)
		return nil
	}

	return nil
}

func applyAdopt(t managedTarget, brainPath string, dryRun bool) error {
	fmt.Printf("  %s  %s  %s\n",
		tui.StyleAccent.Render("adopt "),
		tui.StyleNormal.Render(t.name),
		tui.StyleDim.Render("moving to brain/claude/ and symlinking"),
	)
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(brainPath), 0o755); err != nil {
			return fmt.Errorf("creating brain/claude dir: %w", err)
		}
		if err := os.Rename(t.localPath, brainPath); err != nil {
			return fmt.Errorf("moving %s to brain: %w", t.name, err)
		}
	}
	return makeLink(brainPath, t.localPath, dryRun, t.name)
}

func applyMigrate(t managedTarget, brainPath, oldBrainDir string, dryRun bool) error {
	oldPath := filepath.Join(oldBrainDir, t.brainRel)
	fmt.Printf("  %s  %s  %s\n",
		tui.StyleAccent.Render("migrate"),
		tui.StyleNormal.Render(t.name),
		tui.StyleDim.Render(fmt.Sprintf("moving from brain root to brain/claude/")),
	)
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(brainPath), 0o755); err != nil {
			return fmt.Errorf("creating brain/claude dir: %w", err)
		}
		if err := os.Rename(oldPath, brainPath); err != nil {
			return fmt.Errorf("moving %s to brain/claude/: %w", t.name, err)
		}
		// Remove old symlink before creating new one.
		if err := os.Remove(t.localPath); err != nil {
			return fmt.Errorf("removing old symlink: %w", err)
		}
	}
	return makeLink(brainPath, t.localPath, dryRun, t.name)
}

func applyChildren(t managedTarget, brainPath string, dryRun bool) error {
	entries, err := os.ReadDir(brainPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(t.localPath, 0o755); !dryRun && err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(brainPath, entry.Name())
		dst := filepath.Join(t.localPath, entry.Name())
		if err := makeLink(src, dst, dryRun, t.brainRel+"/"+entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func makeLink(src, dst string, dryRun bool, label string) error {
	info, err := os.Lstat(dst)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			current, _ := os.Readlink(dst)
			if current == src {
				fmt.Printf("  %s  %s\n", tui.StyleMuted.Render("skip  "), tui.StyleDim.Render(label+" already correct"))
				return nil
			}
			fmt.Printf("  %s  %s  %s\n",
				tui.StyleWarn.Render("warn  "),
				tui.StyleNormal.Render(label),
				tui.StyleDim.Render("symlink points elsewhere — skipping"),
			)
			return nil
		}
		// Real file/dir at dst — should not happen if wizard ran correctly, but guard.
		fmt.Printf("  %s  %s  %s\n",
			tui.StyleWarn.Render("warn  "),
			tui.StyleNormal.Render(label),
			tui.StyleDim.Render("non-symlink exists at destination — skipping"),
		)
		return nil
	}

	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("link  "), tui.StyleNormal.Render(label))
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.Symlink(src, dst)
	}
	return nil
}

// checkLink returns true if localPath is a symlink pointing to the expected brainPath.
func checkLink(localPath, brainPath string) bool {
	current, err := os.Readlink(localPath)
	return err == nil && current == brainPath
}

// pathExists returns true if path exists (file or dir).
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// linkStatus returns a human-readable status string for a single managed target.
func linkStatus(t managedTarget, brainClaudeDir string) (ok bool, msg string) {
	brainPath := filepath.Join(brainClaudeDir, t.brainRel)

	if t.kind == linkChildren {
		if !pathExists(brainPath) {
			return true, t.name + " (no brain/claude/" + t.brainRel + "/ — skipped)"
		}
		entries, _ := os.ReadDir(brainPath)
		linked, total := 0, len(entries)
		for _, entry := range entries {
			if checkLink(filepath.Join(t.localPath, entry.Name()), filepath.Join(brainPath, entry.Name())) {
				linked++
			}
		}
		if linked == total {
			return true, fmt.Sprintf("%s (%d/%d linked)", t.name, linked, total)
		}
		return false, fmt.Sprintf("%s (%d/%d linked)", t.name, linked, total)
	}

	if !pathExists(brainPath) {
		return true, t.name + " (not in brain/claude/ — skipped)"
	}
	if checkLink(t.localPath, brainPath) {
		return true, t.name + " → brain/claude/" + t.brainRel
	}
	info, err := os.Lstat(t.localPath)
	if err != nil {
		return false, t.name + " symlink missing"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		current, _ := os.Readlink(t.localPath)
		return false, fmt.Sprintf("%s → wrong target: %s", t.name, current)
	}
	return false, t.name + " exists locally but is not a symlink"
}
