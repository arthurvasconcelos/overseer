package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arthurvasconcelos/overseer/internal/symlink"
	"github.com/spf13/cobra"
)

var dryRun bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Wire dotfiles into their live locations via symlinks",
	Long: `Creates symlinks for dotfiles into their live locations.
Safe to run multiple times — existing correct symlinks are skipped,
real files are backed up to ~/.overseer-backups/<timestamp>/ first.

The repo root is resolved from $OVERSEER_HOME, or the current directory
if the env var is not set.`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without making them")
	rootCmd.AddCommand(setupCmd)
}

func runSetup(_ *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	repoRoot := os.Getenv("OVERSEER_HOME")
	if repoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		repoRoot = cwd
	}

	if dryRun {
		fmt.Println("overseer setup (dry run)")
	} else {
		fmt.Println("overseer setup")
	}
	fmt.Printf("  repo: %s\n\n", repoRoot)

	links := []struct{ src, dst string }{
		{filepath.Join(repoRoot, "dotfiles", "shell", ".zshrc"), filepath.Join(home, ".zshrc")},
		{filepath.Join(repoRoot, "dotfiles", "git", ".gitconfig"), filepath.Join(home, ".gitconfig")},
	}

	for _, l := range links {
		if err := symlink.Make(l.src, l.dst, dryRun); err != nil {
			return fmt.Errorf("symlinking %s: %w", l.dst, err)
		}
	}

	fmt.Println("\nDone.")
	return nil
}
