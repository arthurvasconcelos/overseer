package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

var brainPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull latest changes from the brain's remote",
	RunE:  runBrainPull,
}

var brainDryRun bool

func init() {
	brainSetupCmd.Flags().BoolVar(&brainDryRun, "dry-run", false, "Preview changes without making them")
	brainInitCmd.Hidden = true
	brainCmd.AddCommand(brainInitCmd)
	brainCmd.AddCommand(brainSetupCmd)
	brainCmd.AddCommand(brainStatusCmd)
	brainCmd.AddCommand(brainPullCmd)
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
		filepath.Join(overseerDir, "dotfiles"),
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

	// Version control status.
	fmt.Println()
	if brainIsGit(brainPath) {
		branch := brainGitOutput(brainPath, "rev-parse", "--abbrev-ref", "HEAD")
		remote := cfg.Brain.URL
		if remote == "" {
			remote = brainGitOutput(brainPath, "remote", "get-url", "origin")
		}
		dirty := brainGitOutput(brainPath, "status", "--porcelain")

		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("git repository"))
		if branch != "" {
			fmt.Printf("     %s %s\n", tui.StyleDim.Render("branch:"), tui.StyleNormal.Render(branch))
		}
		if remote != "" {
			fmt.Printf("     %s %s\n", tui.StyleDim.Render("remote:"), tui.StyleNormal.Render(remote))
		}
		if dirty == "" {
			fmt.Printf("     %s %s\n", tui.StyleDim.Render("status:"), tui.StyleOK.Render("clean"))
		} else {
			count := len(strings.Split(strings.TrimSpace(dirty), "\n"))
			fmt.Printf("     %s %s\n", tui.StyleDim.Render("status:"), tui.StyleWarn.Render(fmt.Sprintf("%d uncommitted change(s)", count)))
		}
	} else {
		fmt.Printf("  %s  %s\n", tui.StyleWarn.Render("⚠"), tui.StyleNormal.Render("not a git repository — changes are not versioned"))
		fmt.Printf("     %s\n", tui.StyleMuted.Render("run: cd "+brainPath+" && git init && git remote add origin <url>"))
	}

	// Dotfile wiring count.
	dotfilesDir := filepath.Join(overseerDir, "dotfiles")
	if _, err := os.Stat(dotfilesDir); err == nil {
		total, wired := countDotfiles(dotfilesDir)
		if total > 0 {
			fmt.Println()
			fmt.Printf("  %s %s  %s %s\n",
				tui.StyleDim.Render("dotfiles:"),
				tui.StyleNormal.Render(fmt.Sprintf("%d total", total)),
				tui.StyleDim.Render("/"),
				tui.StyleNormal.Render(fmt.Sprintf("%d wired", wired)),
			)
		}
	}

	if runtime.GOOS == "darwin" {
		fmt.Println()
		localPath, _ := config.LocalPath()
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render("config (local)"), fileLink(localPath))
	}

	return nil
}

func brainIsGit(brainPath string) bool {
	_, err := os.Stat(filepath.Join(brainPath, ".git"))
	return err == nil
}

// brainGitOutput runs a git command in brainPath and returns trimmed stdout.
// Returns empty string on error.
func brainGitOutput(brainPath string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", brainPath}, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func countDotfiles(dotfilesDir string) (total, wired int) {
	home, _ := os.UserHomeDir()
	_ = filepath.WalkDir(dotfilesDir, func(path string, d os.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}
		total++
		rel, _ := filepath.Rel(dotfilesDir, path)
		target := filepath.Join(home, rel)
		if link, err := os.Readlink(target); err == nil && link == path {
			wired++
		}
		return nil
	})
	return
}

// --- brain pull ---

func runBrainPull(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	brainPath := config.ResolveBrainPath(cfg)

	if !brainIsGit(brainPath) {
		fmt.Printf("%s  brain at %s is not a git repository\n", tui.StyleWarn.Render("⚠"), brainPath)
		fmt.Printf("   %s\n", tui.StyleMuted.Render("run: cd "+brainPath+" && git init && git remote add origin <url>"))
		return nil
	}

	remote := cfg.Brain.URL
	if remote == "" {
		remote = brainGitOutput(brainPath, "remote", "get-url", "origin")
	}

	fmt.Println(tui.SectionHeader("brain pull", remote))
	fmt.Println()

	cmd := exec.Command("git", "-C", brainPath, "pull")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
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
