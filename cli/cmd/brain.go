package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/symlink"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/arthurvasconcelos/overseer/internal/wsl"
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

var brainCloneCmd = &cobra.Command{
	Use:   "clone [url]",
	Short: "Clone an existing brain onto this machine",
	Long: `Clones your brain repository into the resolved brain path and applies
brain.git_profile to it.

The URL defaults to brain.url from config. The brain path must be empty or
missing. Under WSL, cloning over SSH offers to point git at ssh.exe first so
the 1Password SSH agent is reachable.

Run overseer setup afterwards to wire dotfiles and machine settings.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBrainClone,
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
var brainStatusFix bool

var brainPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Stage, commit, and push all brain changes",
	Long: `Stages all changes in the brain directory, commits with an auto-generated
message, and pushes to the remote.

Commit message format:
  <n> brain files from <hostname> on <date>
  Affected files:
  <list>`,
	RunE: runBrainPush,
}

var brainGitInitCmd = &cobra.Command{
	Use:   "git-init",
	Short: "Initialize the brain directory as a git repository",
	Long: `Sets up the brain directory as a git repository.

If the brain is already a git repository, this command exits safely with no changes.

Steps:
  1. git init
  2. git remote add origin <url>  (from brain.url in config, or prompted)
  3. Apply brain.git_profile      (if configured)
  4. Initial commit               (if files exist)
  5. git push -u origin <branch>  (if remote is set)`,
	RunE: runBrainGitInit,
}

func init() {
	brainSetupCmd.Flags().BoolVar(&brainDryRun, "dry-run", false, "Preview changes without making them")
	brainStatusCmd.Flags().BoolVar(&brainStatusFix, "fix", false, "Rewrite home paths in the shared config as ~/ and move git.defaults.gpg_ssh_program to its platform overlay")
	brainInitCmd.Hidden = true
	brainCmd.AddCommand(brainInitCmd)
	brainCmd.AddCommand(brainSetupCmd)
	brainCmd.AddCommand(brainCloneCmd)
	brainCmd.AddCommand(brainStatusCmd)
	brainCmd.AddCommand(brainPullCmd)
	brainCmd.AddCommand(brainPushCmd)
	brainCmd.AddCommand(brainGitInitCmd)
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

	fmt.Println()
	fmt.Printf("  %s %s\n", tui.StyleDim.Render("platform:"), tui.StyleNormal.Render(strings.Join(config.PlatformChain(), " → ")))
	overlays := append(config.OverlayConfigPaths(cfg), config.DotfilesDirs(cfg)[1:]...)
	for _, path := range overlays {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("overseer/"+filepath.Base(path)))
		}
	}

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
		fmt.Printf("     %s\n", tui.StyleMuted.Render("run: overseer brain git-init"))
	}

	// Dotfile wiring count.
	if total, wired := countDotfiles(config.DotfilesDirs(cfg)); total > 0 {
		fmt.Println()
		fmt.Printf("  %s %s  %s %s\n",
			tui.StyleDim.Render("dotfiles:"),
			tui.StyleNormal.Render(fmt.Sprintf("%d total", total)),
			tui.StyleDim.Render("/"),
			tui.StyleNormal.Render(fmt.Sprintf("%d wired", wired)),
		)
	}

	if brainStatusFix {
		if err := fixPortability(cfg); err != nil {
			return err
		}
	}
	printPortability(overseerDir)

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

func countDotfiles(dirs []string) (total, wired int) {
	home, _ := os.UserHomeDir()
	sources, _ := symlink.Collect(dirs)
	for rel, source := range sources {
		total++
		if link, err := os.Readlink(filepath.Join(home, rel)); err == nil && link == source {
			wired++
		}
	}
	return
}

// printPortability warns about shared brain content that only works on one
// platform: absolute paths in config.yaml and hardcoded paths in dotfiles/.
func printPortability(overseerDir string) {
	issues, err := config.NonPortablePaths(filepath.Join(overseerDir, "config.yaml"))
	if err != nil {
		return
	}

	type dotfileHit struct {
		rel   string
		lines []int
	}
	var hits []dotfileHit
	dotfilesDir := filepath.Join(overseerDir, "dotfiles")
	if sources, err := symlink.Collect([]string{dotfilesDir}); err == nil {
		for rel, source := range sources {
			if lines := config.PlatformSpecificLines(source); len(lines) > 0 {
				hits = append(hits, dotfileHit{rel, lines})
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].rel < hits[j].rel })

	if len(issues) == 0 && len(hits) == 0 {
		return
	}

	fmt.Println()
	fmt.Printf("  %s  %s\n", tui.StyleWarn.Render("⚠"), tui.StyleNormal.Render("shared brain content that only works on some platforms"))
	for _, issue := range issues {
		fmt.Printf("     %s %s %s\n",
			tui.StyleDim.Render(fmt.Sprintf("config.yaml:%d", issue.Line)),
			tui.StyleNormal.Render(issue.Key),
			tui.StyleMuted.Render(issue.Value))
	}
	for _, hit := range hits {
		lines := make([]string, len(hit.lines))
		for i, n := range hit.lines {
			lines[i] = strconv.Itoa(n)
		}
		fmt.Printf("     %s %s\n",
			tui.StyleDim.Render("dotfiles/"+hit.rel),
			tui.StyleMuted.Render("line "+strings.Join(lines, ", ")))
	}
	fmt.Printf("     %s\n", tui.StyleMuted.Render("use ~/ paths, or move these to config.<platform>.yaml / dotfiles.<platform>/"))
	if len(issues) > 0 && !brainStatusFix {
		fmt.Printf("     %s\n", tui.StyleMuted.Render("run: overseer brain status --fix  (fixes home paths and git.defaults.gpg_ssh_program)"))
	}
}

func fixPortability(cfg *config.Config) error {
	fixes, err := config.FixPortability(cfg)
	if err != nil {
		return err
	}
	fmt.Println()
	if len(fixes) == 0 {
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("nothing to fix automatically"))
		return nil
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render(fmt.Sprintf("fixed %d value(s) in config.yaml", len(fixes))))
	for _, fix := range fixes {
		fmt.Printf("     %s %s %s %s\n",
			tui.StyleNormal.Render(fix.Key),
			tui.StyleMuted.Render(fix.From),
			tui.StyleDim.Render("→"),
			tui.StyleNormal.Render(fix.To))
	}
	fmt.Printf("     %s\n", tui.StyleMuted.Render("review with: git -C "+config.ResolveBrainPath(cfg)+" diff"))
	return nil
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
		fmt.Printf("   %s\n", tui.StyleMuted.Render("run: overseer brain git-init"))
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

// --- brain git-init ---

func runBrainGitInit(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	brainPath := config.ResolveBrainPath(cfg)

	if brainIsGit(brainPath) {
		fmt.Printf("%s  brain at %s is already a git repository\n", tui.StyleOK.Render("✓"), brainPath)
		return nil
	}

	fmt.Println(tui.SectionHeader("brain git-init", brainPath))
	fmt.Println()

	if err := initBrainRepo(cfg, cfg.Brain.URL); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("%s  brain is now a git repository\n", tui.StyleOK.Render("✓"))
	return nil
}

// initBrainRepo initializes the brain directory as a new git repository,
// sets the remote, applies the git profile, makes an initial commit, and pushes.
// remoteURL may be empty — if so, the user is prompted.
func initBrainRepo(cfg *config.Config, remoteURL string) error {
	brainPath := config.ResolveBrainPath(cfg)

	if remoteURL == "" {
		var err error
		remoteURL, err = tui.Prompt("remote URL (leave blank to skip)", "", "")
		if err != nil {
			return err
		}
		fmt.Println()
	}

	if out, err := gitIn(brainPath, "init"); err != nil {
		return fmt.Errorf("git init: %s", strings.TrimSpace(out))
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("initialized git repository"))

	if remoteURL != "" {
		if out, err := gitIn(brainPath, "remote", "add", "origin", remoteURL); err != nil {
			return fmt.Errorf("git remote add: %s", strings.TrimSpace(out))
		}
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("remote set to "+remoteURL))
	}

	applyBrainGitProfile(brainPath, cfg)

	porcelain := brainGitOutput(brainPath, "status", "--porcelain")
	if porcelain != "" {
		hostname, _ := os.Hostname()
		date := time.Now().Format("Mon, 02 Jan 2006 15:04")
		msg := fmt.Sprintf("initial brain commit from %s on %s", hostname, date)

		if out, err := gitIn(brainPath, "add", "-A"); err != nil {
			return fmt.Errorf("git add: %s", strings.TrimSpace(out))
		}
		if out, err := gitIn(brainPath, "commit", "-m", msg); err != nil {
			return fmt.Errorf("git commit: %s", strings.TrimSpace(out))
		}
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("created initial commit"))
	}

	if remoteURL != "" {
		branch := brainGitOutput(brainPath, "rev-parse", "--abbrev-ref", "HEAD")
		if branch == "" {
			branch = "main"
		}
		cmd := exec.Command("git", "-C", brainPath, "push", "-u", "origin", branch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git push failed: %w", err)
		}
	}

	return nil
}

// cloneBrain clones remoteURL into brainPath. brainPath must be missing or
// empty; an existing git repository there is kept as is.
func cloneBrain(remoteURL, brainPath string) error {
	if brainIsGit(brainPath) {
		origin := brainGitOutput(brainPath, "remote", "get-url", "origin")
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("brain already cloned at "+brainPath))
		if origin != "" && origin != remoteURL {
			fmt.Println(tui.WarnLine("brain", "its origin is "+origin+", not "+remoteURL))
		}
		return nil
	}
	if entries, err := os.ReadDir(brainPath); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s exists and is not empty — move it aside or choose another brain path", brainPath)
	}

	prepareWSLSSH(remoteURL)

	if err := os.MkdirAll(filepath.Dir(brainPath), 0755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(brainPath), err)
	}
	fmt.Printf("  %s  %s\n", tui.StyleNormal.Render("↓"), tui.StyleNormal.Render("cloning "+remoteURL))
	cmd := exec.Command("git", "clone", remoteURL, brainPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if wsl.Detected() && isSSHRemote(remoteURL) {
			return fmt.Errorf("git clone failed: %w — under WSL, check that core.sshCommand is ssh.exe (see the WSL guide)", err)
		}
		return fmt.Errorf("git clone failed: %w", err)
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("cloned brain into "+brainPath))
	return nil
}

// prepareWSLSSH offers to point git at ssh.exe before an SSH clone under WSL,
// because the Linux ssh can't reach the 1Password agent. An existing
// core.sshCommand is never changed.
func prepareWSLSSH(remoteURL string) {
	if !wsl.Detected() || !isSSHRemote(remoteURL) {
		return
	}
	if current, _ := exec.Command("git", "config", "--global", "core.sshCommand").Output(); strings.TrimSpace(string(current)) != "" {
		return
	}
	if _, err := exec.LookPath("ssh.exe"); err != nil {
		fmt.Println(tui.WarnLine("ssh", "ssh.exe not on PATH — git will use the Linux ssh, which can't reach the 1Password agent"))
		return
	}
	confirmed, err := tui.Confirm("git uses the Linux ssh, which can't reach the 1Password agent — set core.sshCommand to ssh.exe globally?")
	if err != nil || !confirmed {
		return
	}
	if out, err := exec.Command("git", "config", "--global", "core.sshCommand", "ssh.exe").CombinedOutput(); err != nil {
		fmt.Println(tui.WarnLine("ssh", "setting core.sshCommand: "+strings.TrimSpace(string(out))))
		return
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("git core.sshCommand set to ssh.exe"))
}

// isSSHRemote reports whether a git remote URL uses SSH (ssh:// or scp-like user@host:path).
func isSSHRemote(url string) bool {
	if strings.HasPrefix(url, "ssh://") {
		return true
	}
	return !strings.Contains(url, "://") && strings.Contains(url, "@") && strings.Contains(url, ":")
}

// --- brain clone ---

func runBrainClone(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	remoteURL := cfg.Brain.URL
	if len(args) == 1 {
		remoteURL = args[0]
	}
	if remoteURL == "" {
		remoteURL, err = tui.Prompt("brain remote URL", "", "")
		if err != nil {
			return err
		}
		if remoteURL == "" {
			return fmt.Errorf("a remote URL is required to clone a brain")
		}
	}

	brainPath := config.ResolveBrainPath(cfg)
	fmt.Println(tui.SectionHeader("brain clone", brainPath))
	fmt.Println()

	if err := cloneBrain(remoteURL, brainPath); err != nil {
		return err
	}

	cfg, err = config.Load()
	if err != nil {
		return err
	}
	applyBrainGitProfile(brainPath, cfg)

	fmt.Println()
	fmt.Println(tui.StyleMuted.Render("Next: overseer setup — machine settings, git profile and dotfiles"))
	return nil
}

// applyBrainGitProfile applies brain.git_profile to the brain repo if configured.
func applyBrainGitProfile(brainPath string, cfg *config.Config) {
	if cfg.Brain.GitProfile == "" {
		return
	}
	for i := range cfg.Git.Profiles {
		if cfg.Git.Profiles[i].Name == cfg.Brain.GitProfile {
			resolved, err := resolveProfile(cfg.Git.Profiles[i], cfg.Git.Defaults, cfg.System)
			if err != nil {
				fmt.Printf("  %s  resolving git profile %q: %v\n", tui.StyleWarn.Render("⚠"), cfg.Brain.GitProfile, err)
			} else if err := applyGitConfigIn(brainPath, gitScopeLocal, resolved); err != nil {
				fmt.Printf("  %s  applying git profile %q: %v\n", tui.StyleWarn.Render("⚠"), cfg.Brain.GitProfile, err)
			} else {
				fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("applied git profile "+cfg.Brain.GitProfile))
			}
			return
		}
	}
}

// --- brain push ---

func runBrainPush(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	brainPath := config.ResolveBrainPath(cfg)

	if !brainIsGit(brainPath) {
		fmt.Printf("%s  brain at %s is not a git repository\n", tui.StyleWarn.Render("⚠"), brainPath)
		fmt.Printf("   %s\n", tui.StyleMuted.Render("run: overseer brain git-init"))
		return nil
	}

	porcelain := brainGitOutput(brainPath, "status", "--porcelain")
	upstream := brainGitOutput(brainPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if porcelain == "" && !brainHasUnpushed(brainPath, upstream) {
		fmt.Printf("%s  nothing to push, brain is up to date\n", tui.StyleOK.Render("✓"))
		return nil
	}

	remote := cfg.Brain.URL
	if remote == "" {
		remote = brainGitOutput(brainPath, "remote", "get-url", "origin")
	}
	fmt.Println(tui.SectionHeader("brain push", remote))
	fmt.Println()

	if porcelain != "" {
		if err := commitBrain(brainPath, cfg, porcelain); err != nil {
			return err
		}
	}

	pushArgs := []string{"-C", brainPath, "push"}
	if upstream == "" {
		if brainGitOutput(brainPath, "remote", "get-url", "origin") == "" {
			if cfg.Brain.URL == "" {
				return fmt.Errorf("brain has no origin remote — set brain.url in config.yaml or run: git -C %s remote add origin <url>", brainPath)
			}
			if out, err := gitIn(brainPath, "remote", "add", "origin", cfg.Brain.URL); err != nil {
				return fmt.Errorf("git remote add: %s", strings.TrimSpace(out))
			}
		}
		pushArgs = append(pushArgs, "--set-upstream", "origin", "HEAD")
	}

	cmd := exec.Command("git", pushArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	return nil
}

func commitBrain(brainPath string, cfg *config.Config, porcelain string) error {
	// Parse affected file names from porcelain output.
	lines := strings.Split(strings.TrimSpace(porcelain), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		name := strings.TrimSpace(line[3:])
		// Renames: "old -> new" — keep only the destination.
		if idx := strings.Index(name, " -> "); idx != -1 {
			name = name[idx+4:]
		}
		files = append(files, name)
	}

	applyBrainGitProfile(brainPath, cfg)

	hostname, _ := os.Hostname()
	date := time.Now().Format("Mon, 02 Jan 2006 15:04")
	msg := fmt.Sprintf("%d brain files from %s on %s\nAffected files:\n%s",
		len(files), hostname, date, strings.Join(files, "\n"))

	if out, err := gitIn(brainPath, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %s", strings.TrimSpace(out))
	}

	if out, err := gitIn(brainPath, "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %s", strings.TrimSpace(out))
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render(strings.SplitN(msg, "\n", 2)[0]))
	fmt.Println()
	return nil
}

// brainHasUnpushed reports whether the brain has local commits that the
// upstream lacks. With no upstream, any commit counts as unpushed.
func brainHasUnpushed(brainPath, upstream string) bool {
	if brainGitOutput(brainPath, "rev-parse", "--verify", "HEAD") == "" {
		return false
	}
	if upstream == "" {
		return true
	}
	return brainGitOutput(brainPath, "rev-list", "--count", upstream+"..HEAD") != "0"
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

# repo_dirs:
#   - ~/repos/work

# obsidian:
#   vault_path: ~/path/to/your/vault
#   vault_name: MyVault
#   daily_notes_folder: Daily
#   templates_folder: Templates

# Paths: prefer ~/... so this file works on every machine. Values that differ
# per platform go in config.darwin.yaml, config.linux.yaml or config.wsl.yaml
# next to this file (WSL also reads config.linux.yaml). Dotfiles work the same
# way: dotfiles.<platform>/ overrides files in dotfiles/.
`
