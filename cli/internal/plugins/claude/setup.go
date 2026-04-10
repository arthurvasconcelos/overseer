package claude

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
		Short: "Set up Claude config symlinks from brain to ~/.claude/",
		Long: `Interactive wizard that adopts existing Claude configuration into the brain
and creates symlinks from ~/.claude/ back to the brain.

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
		Short: "List Claude config symlink status",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runList(cfg)
		},
	}
}

func runList(cfg *config.Config) error {
	claudeDir := brainClaudeDir(cfg)
	targets := wellKnownTargets(claudeDir)

	fmt.Println(tui.SectionHeader("claude config", claudeDir))
	fmt.Println()

	maxLen := 0
	for _, t := range targets {
		if len(t.name) > maxLen {
			maxLen = len(t.name)
		}
	}

	allOK := true
	for _, t := range targets {
		ok, msg := linkStatus(t, claudeDir)
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
		fmt.Println("  " + tui.StyleWarn.Render("some links need attention — run: overseer claude setup"))
	}
	return nil
}

func runSetup(cfg *config.Config, dryRun bool) error {
	claudeDir := brainClaudeDir(cfg)
	oldBrainRoot := config.ResolveBrainPath(cfg)
	targets := wellKnownTargets(claudeDir)

	fmt.Println(tui.SectionHeader("overseer claude setup", ""))
	fmt.Println()
	if dryRun {
		fmt.Println("  " + tui.StyleWarn.Render("dry run — no changes will be made"))
		fmt.Println()
	}

	fmt.Println(tui.StyleDim.Render("Scanning Claude configuration..."))
	fmt.Println()

	var scans []targetScan
	for _, t := range targets {
		scan := scanTarget(t, claudeDir, oldBrainRoot)
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

	// Create brain/claude/ directory.
	if !dryRun {
		if err := os.MkdirAll(claudeDir, 0o755); err != nil {
			return fmt.Errorf("creating brain/claude dir: %w", err)
		}
	}

	// Apply each scan in order. Stop on first error.
	for _, s := range scans {
		if err := applyTarget(s, claudeDir, oldBrainRoot, dryRun); err != nil {
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
