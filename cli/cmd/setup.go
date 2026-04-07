package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/symlink"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var dryRun bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Wire dotfiles and install Brew packages from your brain",
	Long: `Creates symlinks for dotfiles from your brain into their live locations,
then installs missing Homebrew packages from your brain's Brewfile (macOS only).

Safe to run multiple times — existing correct symlinks are skipped,
real files are backed up to ~/.overseer-backups/<timestamp>/ first.`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without making them")
	rootCmd.AddCommand(setupCmd)
}

func runSetup(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runBrainSetup(cfg, dryRun)
}

// runBrainSetup wires dotfiles from brain and installs Brew packages.
// Shared by overseer setup and overseer brain setup.
func runBrainSetup(cfg *config.Config, dry bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	brainOverseer := config.BrainOverseerPath(cfg)

	if dry {
		fmt.Println("overseer setup (dry run)")
	} else {
		fmt.Println("overseer setup")
	}
	fmt.Printf("  brain: %s\n\n", config.ResolveBrainPath(cfg))

	dotfilesDir := fmt.Sprintf("%s/dotfiles", brainOverseer)
	if _, err := os.Stat(dotfilesDir); os.IsNotExist(err) {
		fmt.Println(tui.WarnLine("setup", "dotfiles not found in brain — run: overseer brain init"))
	} else {
		if err := symlink.MakeAll(dotfilesDir, home, dry); err != nil {
			return fmt.Errorf("wiring dotfiles: %w", err)
		}
	}

	// Brew install — macOS only, skipped on other platforms.
	if runtime.GOOS == "darwin" && brewAvailable() {
		fmt.Println()
		if err := runBrewInstall(nil, nil); err != nil {
			fmt.Println(tui.WarnLine("brew", err.Error()))
		}
	}

	fmt.Println("\nDone.")
	return nil
}
