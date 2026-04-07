package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var brainCmd = &cobra.Command{
	Use:   "brain",
	Short: "Manage your brain directory",
}

var brainInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold the brain directory structure",
	RunE:  runBrainInit,
}

var brainSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Wire dotfiles and install Brew packages from your brain",
	Long: `Creates symlinks for dotfiles from your brain into their live locations,
then installs missing Homebrew packages from your brain's Brewfile (macOS only).

Safe to run multiple times — existing correct symlinks are skipped,
real files are backed up to ~/.overseer-backups/<timestamp>/ first.`,
	RunE: runBrainSetupCmd,
}

var brainStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show brain directory health",
	RunE:  runBrainStatus,
}

var brainPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the resolved brain path",
	RunE:  runBrainPath,
}

var brainDryRun bool

func init() {
	brainSetupCmd.Flags().BoolVar(&brainDryRun, "dry-run", false, "Preview changes without making them")
	brainCmd.AddCommand(brainInitCmd)
	brainCmd.AddCommand(brainSetupCmd)
	brainCmd.AddCommand(brainStatusCmd)
	brainCmd.AddCommand(brainPathCmd)
	rootCmd.AddCommand(brainCmd)
}

// --- brain init ---

func runBrainInit(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	brainPath := config.ResolveBrainPath(cfg)
	overseerDir := config.BrainOverseerPath(cfg)

	fmt.Printf("overseer brain init\n")
	fmt.Printf("  brain: %s\n\n", brainPath)

	dirs := []string{
		filepath.Join(overseerDir, "dotfiles", "shell"),
		filepath.Join(overseerDir, "dotfiles", "git"),
		filepath.Join(overseerDir, "plugins"),
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			fmt.Printf("  [skip]   %s already exists\n", dir)
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Printf("  [mkdir]  %s\n", dir)
	}

	brewfile := filepath.Join(overseerDir, "Brewfile")
	if _, err := os.Stat(brewfile); os.IsNotExist(err) {
		if err := os.WriteFile(brewfile, []byte("# Add your Homebrew packages here\n"), 0644); err != nil {
			return fmt.Errorf("creating Brewfile: %w", err)
		}
		fmt.Printf("  [create] %s\n", brewfile)
	} else {
		fmt.Printf("  [skip]   %s already exists\n", brewfile)
	}

	brewfileLocal := filepath.Join(overseerDir, "Brewfile.local.example")
	if _, err := os.Stat(brewfileLocal); os.IsNotExist(err) {
		content := "# Machine-specific packages — copy to Brewfile.local (gitignored)\n"
		if err := os.WriteFile(brewfileLocal, []byte(content), 0644); err != nil {
			return fmt.Errorf("creating Brewfile.local.example: %w", err)
		}
		fmt.Printf("  [create] %s\n", brewfileLocal)
	} else {
		fmt.Printf("  [skip]   %s already exists\n", brewfileLocal)
	}

	configFile := filepath.Join(overseerDir, "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := os.WriteFile(configFile, []byte(brainConfigExample), 0644); err != nil {
			return fmt.Errorf("creating config.yaml: %w", err)
		}
		fmt.Printf("  [create] %s\n", configFile)
	} else {
		fmt.Printf("  [skip]   %s already exists\n", configFile)
	}

	fmt.Println()
	fmt.Println("Done. Next steps:")
	fmt.Println("  1. Edit " + configFile)
	fmt.Println("  2. Add your dotfiles under " + filepath.Join(overseerDir, "dotfiles") + "/")
	fmt.Println("  3. Run: overseer brain setup")
	return nil
}

// --- brain setup ---

func runBrainSetupCmd(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runBrainSetup(cfg, brainDryRun)
}

// --- brain status ---

func runBrainStatus(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	brainPath := config.ResolveBrainPath(cfg)
	overseerDir := config.BrainOverseerPath(cfg)

	fmt.Println(tui.SectionHeader("brain status", brainPath))
	fmt.Println()

	checkPath := func(label, path string) {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render(label))
		} else {
			fmt.Printf("  %s  %s\n", tui.StyleError.Render("✗"), tui.StyleMuted.Render(label+" (missing)"))
		}
	}

	checkPath("brain directory", brainPath)
	checkPath("overseer/config.yaml", filepath.Join(overseerDir, "config.yaml"))
	checkPath("overseer/dotfiles/", filepath.Join(overseerDir, "dotfiles"))
	checkPath("overseer/Brewfile", filepath.Join(overseerDir, "Brewfile"))
	checkPath("overseer/plugins/", filepath.Join(overseerDir, "plugins"))

	// Count wired dotfiles.
	dotfilesDir := filepath.Join(overseerDir, "dotfiles")
	if _, err := os.Stat(dotfilesDir); err == nil {
		count := 0
		_ = filepath.WalkDir(dotfilesDir, func(_ string, d os.DirEntry, _ error) error {
			if !d.IsDir() {
				count++
			}
			return nil
		})
		if count > 0 {
			home, _ := os.UserHomeDir()
			wired := 0
			_ = filepath.WalkDir(dotfilesDir, func(path string, d os.DirEntry, _ error) error {
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(dotfilesDir, path)
				target := filepath.Join(home, rel)
				if link, err := os.Readlink(target); err == nil && link == path {
					wired++
				}
				return nil
			})
			fmt.Printf("\n  dotfiles: %d total, %d wired\n",
				tui.StyleNormal.Render(fmt.Sprintf("%d", count)),
				tui.StyleNormal.Render(fmt.Sprintf("%d", wired)),
			)
		}
	}

	// macOS only: brew status.
	if runtime.GOOS == "darwin" {
		fmt.Println()
		localPath, _ := config.LocalPath()
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render("config (local)"), fileLink(localPath))
	}

	return nil
}

// --- brain path ---

func runBrainPath(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	fmt.Println(config.ResolveBrainPath(cfg))
	return nil
}

// brainConfigExample is written to brain/overseer/config.yaml on brain init.
const brainConfigExample = `# overseer configuration
# Full reference: https://github.com/arthurvasconcelos/overseer

# secrets:
#   environments:
#     work: <1password-account-id>   # see: overseer accounts

# integrations:
#   jira:
#     - name: work
#       base_url: https://yourorg.atlassian.net
#       email: op://Vault/Jira/username
#       token: op://Vault/Jira/credential
#       op_account: <1password-account-id>
#
#   slack:
#     - name: work
#       token: op://Vault/Slack Bot Token/credential
#       op_account: <1password-account-id>
#
#   github:
#     - name: personal
#       token: op://Vault/GitHub PAT/credential
#
#   gitlab:
#     - name: work
#       base_url: https://gitlab.yourorg.com
#       token: op://Vault/GitLab PAT/credential

# git:
#   defaults:
#     gpg_format: ssh
#     commit_gpgsign: true
#   profiles:
#     - name: personal
#       email: you@example.com
#       signing_key: op://Vault/SSH Key/public key

# repos:
#   - name: my-project
#     url: git@github.com:you/my-project.git
#     path: repos/my-project
#     git_profile: personal

# obsidian:
#   vault_path: /absolute/path/to/your/vault
#   vault_name: MyVault
#   daily_notes_folder: Daily
#   templates_folder: Templates
`
