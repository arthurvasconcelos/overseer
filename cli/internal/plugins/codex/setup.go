package codex

import (
	"fmt"
	"os"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

func setupCmd(cfg *config.Config) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up Codex config symlinks from brain to ~/.codex/",
		Long: `Interactive wizard that adopts existing Codex configuration into the brain
and creates symlinks from ~/.codex/ back to the brain.

Safe to run multiple times — already correct symlinks are skipped.
Use --dry-run to preview changes without applying them.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSetup(cfg, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")
	return cmd
}

func listCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Codex config symlink status",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runList(cfg)
		},
	}
}

func runList(cfg *config.Config) error {
	codexDir := brainCodexDir(cfg)
	targets := wellKnownTargets(codexDir)

	fmt.Println(tui.SectionHeader("codex config", codexDir))
	fmt.Println()

	maxLen := 0
	for _, t := range targets {
		if len(t.name) > maxLen {
			maxLen = len(t.name)
		}
	}

	allOK := true
	for _, t := range targets {
		ok, msg := linkStatus(t, codexDir)
		icon := tui.StyleOK.Render("✓")
		if !ok {
			icon = tui.StyleError.Render("✗")
			allOK = false
		}
		padding := strings.Repeat(" ", maxLen-len(t.name)+2)
		fmt.Printf("  %s%s%s  %s\n",
			tui.StyleNormal.Render(t.name),
			padding,
			icon,
			tui.StyleDim.Render(msg),
		)
	}

	fmt.Println()
	if allOK {
		fmt.Println("  " + tui.StyleOK.Render("all links healthy"))
	} else {
		fmt.Println("  " + tui.StyleWarn.Render("some links need attention — run: overseer codex setup"))
	}
	return nil
}

func runSetup(cfg *config.Config, dryRun bool) error {
	codexDir := brainCodexDir(cfg)
	oldBrainRoot := config.ResolveBrainPath(cfg)
	targets := wellKnownTargets(codexDir)

	fmt.Println(tui.SectionHeader("overseer codex setup", ""))
	fmt.Println()
	if dryRun {
		fmt.Println("  " + tui.StyleWarn.Render("dry run — no changes will be made"))
		fmt.Println()
	}

	fmt.Println(tui.StyleDim.Render("Scanning Codex configuration..."))
	fmt.Println()

	var scans []targetScan
	for _, t := range targets {
		scan := scanTarget(t, codexDir, oldBrainRoot)
		scans = append(scans, scan)
	}

	// Print scan table.
	nameWidth := 0
	detailWidth := 0
	for _, s := range scans {
		if len(s.target.name) > nameWidth {
			nameWidth = len(s.target.name)
		}
		if len(actionLabel(s.action)) > detailWidth {
			detailWidth = len(actionLabel(s.action))
		}
	}

	hasWork := false
	for _, s := range scans {
		if s.action != actionSkip && s.action != actionConflict {
			hasWork = true
		}
		namePad := strings.Repeat(" ", nameWidth-len(s.target.name)+2)
		actionPad := strings.Repeat(" ", detailWidth-len(actionLabel(s.action))+2)
		fmt.Printf("  %s%s%s%s%s\n",
			tui.StyleNormal.Render(s.target.name),
			namePad,
			actionStyle(s.action)(actionLabel(s.action)),
			actionPad,
			tui.StyleDim.Render(s.detail),
		)
	}
	fmt.Println()

	if !hasWork {
		fmt.Println("  " + tui.StyleOK.Render("nothing to do — all links are already in place"))
		return nil
	}

	if !dryRun {
		confirmed, err := tui.Confirm("Ready to proceed?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(tui.StyleMuted.Render("aborted"))
			return nil
		}
		fmt.Println()
	}

	// Create brain/codex/ directory.
	if !dryRun {
		if err := os.MkdirAll(codexDir, 0o755); err != nil {
			return fmt.Errorf("creating brain/codex dir: %w", err)
		}
	}

	state := readSkillsState(codexDir)

	// Apply each scan in order. Stop on first error.
	for _, s := range scans {
		var skipFn func(string) bool
		if s.target.brainRel == "skills" {
			skipFn = func(name string) bool { return isDisabled(state, name) }
		}
		if err := applyTarget(s, codexDir, oldBrainRoot, dryRun, skipFn); err != nil {
			return fmt.Errorf("%s: %w", s.target.name, err)
		}
	}

	fmt.Println()
	if dryRun {
		fmt.Println("  " + tui.StyleMuted.Render("dry run complete — no changes were made"))
	} else {
		fmt.Println("  " + tui.StyleOK.Render("done"))
	}
	return nil
}

func actionLabel(a targetAction) string {
	switch a {
	case actionSkip:
		return "skip"
	case actionLinkOnly:
		return "link"
	case actionAdopt:
		return "adopt"
	case actionMigrate:
		return "migrate"
	case actionConflict:
		return "conflict"
	default:
		return "?"
	}
}

func actionStyle(a targetAction) func(string) string {
	switch a {
	case actionSkip:
		return func(s string) string { return tui.StyleMuted.Render(s) }
	case actionLinkOnly:
		return func(s string) string { return tui.StyleOK.Render(s) }
	case actionAdopt, actionMigrate:
		return func(s string) string { return tui.StyleAccent.Render(s) }
	case actionConflict:
		return func(s string) string { return tui.StyleWarn.Render(s) }
	default:
		return func(s string) string { return tui.StyleDim.Render(s) }
	}
}
