package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Manage overseer-controlled repositories",
}

var reposStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show git status across all managed repos",
	RunE:  runReposStatus,
}

var reposPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull latest changes across all managed repos (clones if missing)",
	RunE:  runReposPull,
}

var reposSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Apply git profiles to all already-cloned managed repos",
	RunE:  runReposSetup,
}

var reposOpenCmd = &cobra.Command{
	Use:   "open [name]",
	Short: "Open a managed repo in browser, Finder, or IDE",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runReposOpen,
}

var (
	reposOpenBrowser bool
	reposOpenFinder  bool
	reposOpenIDE     bool
)

// sshRemoteRe matches SSH-format git remotes: git@host:path.git
var sshRemoteRe = regexp.MustCompile(`^git@([^:]+):(.+?)(?:\.git)?$`)

func init() {
	reposOpenCmd.Flags().BoolVar(&reposOpenBrowser, "browser", false, "Open repo URL in browser (default)")
	reposOpenCmd.Flags().BoolVar(&reposOpenFinder, "finder", false, "Reveal local path in Finder")
	reposOpenCmd.Flags().BoolVar(&reposOpenIDE, "ide", false, "Open local path in IDE")
	reposCmd.AddCommand(reposStatusCmd)
	reposCmd.AddCommand(reposPullCmd)
	reposCmd.AddCommand(reposSetupCmd)
	reposCmd.AddCommand(reposOpenCmd)
	rootCmd.AddCommand(reposCmd)
}

// --- open ---

func runReposOpen(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Repos) == 0 {
		fmt.Println(tui.StyleMuted.Render("no repos configured — add entries under repos: in config.yaml"))
		return nil
	}

	// Pick repo: from arg or interactive selector.
	var repo config.RepoConfig
	if len(args) > 0 {
		name := args[0]
		found := false
		for _, r := range cfg.Repos {
			if r.Name == name {
				repo = r
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no repo named %q", name)
		}
	} else {
		items := make([]tui.SelectItem, len(cfg.Repos))
		for i, r := range cfg.Repos {
			items[i] = tui.SelectItem{Title: r.Name, Subtitle: r.Path}
		}
		idx, err := tui.Select("select repo", items)
		if err != nil {
			return err
		}
		if idx == -1 {
			return nil
		}
		repo = cfg.Repos[idx]
	}

	home := resolveReposPath(cfg)
	localPath := repoRoot(home, repo.Path)

	// Default to --browser if no flag is set.
	if !reposOpenBrowser && !reposOpenFinder && !reposOpenIDE {
		reposOpenBrowser = true
	}

	if reposOpenBrowser {
		browserURL := repoToHTTPS(repo.URL)
		if browserURL == "" {
			return fmt.Errorf("could not derive browser URL from remote: %s", repo.URL)
		}
		fmt.Println(tui.StyleMuted.Render("opening ") + tui.StyleAccent.Render(browserURL))
		return exec.Command("open", browserURL).Run()
	}

	if reposOpenFinder {
		fmt.Println(tui.StyleMuted.Render("revealing in Finder: ") + tui.StyleAccent.Render(localPath))
		return exec.Command("open", "-R", localPath).Run()
	}

	if reposOpenIDE {
		ide := repo.IDE
		if ide == "" {
			ide = cfg.System.IDE
		}
		if ide == "" {
			return fmt.Errorf("no IDE configured — set system.ide in config.local.yaml or repos[%s].ide", repo.Name)
		}
		fmt.Println(tui.StyleMuted.Render("opening in "+ide+": ") + tui.StyleAccent.Render(localPath))
		return exec.Command(ide, localPath).Run()
	}

	return nil
}

// repoToHTTPS converts a git remote URL (SSH or HTTPS) to an HTTPS browser URL.
// git@github.com:owner/repo.git → https://github.com/owner/repo
// https://github.com/owner/repo.git → https://github.com/owner/repo
func repoToHTTPS(remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	if m := sshRemoteRe.FindStringSubmatch(remote); len(m) == 3 {
		return "https://" + m[1] + "/" + m[2]
	}
	if strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "http://") {
		return remote
	}
	return ""
}

// repoRoot returns the absolute path for a repo given the overseer home dir.
func repoRoot(overseerHome, repoPath string) string {
	if filepath.IsAbs(repoPath) {
		return repoPath
	}
	return filepath.Join(overseerHome, repoPath)
}

// resolveReposPath returns the overseer repo root using this precedence:
// 1. OVERSEER_REPOS_PATH env var
// 2. system.repos_path in config.local.yaml
// 3. parent of the directory containing the binary (best-effort)
func resolveReposPath(cfg *config.Config) string {
	if h := os.Getenv("OVERSEER_REPOS_PATH"); h != "" {
		return h
	}
	if cfg.System.ReposPath != "" {
		return cfg.System.ReposPath
	}
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	// binary lives at <overseer-home>/bin/overseer — go up two levels
	return filepath.Dir(filepath.Dir(exe))
}

type repoResult struct {
	name     string
	readonly bool
	output   string
	err      error
}

type repoStatusJSON struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Readonly bool     `json:"readonly"`
	Cloned   bool     `json:"cloned"`
	Branch   string   `json:"branch,omitempty"`
	Clean    bool     `json:"clean"`
	Changes  []string `json:"changes"`
	Error    string   `json:"error,omitempty"`
}

func runReposStatus(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Repos) == 0 {
		if outputFormat == "json" {
			return printJSON([]repoStatusJSON{})
		}
		fmt.Println(tui.StyleMuted.Render("no repos configured — add entries under repos: in config.yaml"))
		return nil
	}

	home := resolveReposPath(cfg)
	results := make([]repoResult, len(cfg.Repos))
	var wg sync.WaitGroup

	for i, repo := range cfg.Repos {
		i, repo := i, repo
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = repoStatus(home, repo)
		}()
	}
	wg.Wait()

	if outputFormat == "json" {
		out := make([]repoStatusJSON, len(cfg.Repos))
		for i, repo := range cfg.Repos {
			path := repoRoot(home, repo.Path)
			r := results[i]
			s := repoStatusJSON{
				Name:     repo.Name,
				Path:     path,
				Readonly: repo.Readonly,
				Changes:  []string{},
			}
			if r.err != nil {
				s.Error = r.err.Error()
				out[i] = s
				continue
			}
			raw, err := gitIn(path, "status", "--short", "--branch")
			if err != nil {
				s.Error = err.Error()
				out[i] = s
				continue
			}
			s.Cloned = true
			for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "## ") {
					s.Branch = strings.TrimPrefix(strings.SplitN(line[3:], "...", 2)[0], "No commits yet on ")
				} else {
					s.Changes = append(s.Changes, line)
				}
			}
			s.Clean = len(s.Changes) == 0
			out[i] = s
		}
		return printJSON(out)
	}

	for _, r := range results {
		if r.err != nil {
			fmt.Println(tui.WarnLine(r.name, r.err.Error()))
			fmt.Println()
		} else {
			fmt.Print(r.output)
		}
	}
	return nil
}

func runReposPull(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Repos) == 0 {
		fmt.Println(tui.StyleMuted.Render("no repos configured — add entries under repos: in config.yaml"))
		return nil
	}

	home := resolveReposPath(cfg)
	results := make([]repoResult, len(cfg.Repos))
	var wg sync.WaitGroup

	for i, repo := range cfg.Repos {
		i, repo := i, repo
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = repoPull(home, repo, cfg)
		}()
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			fmt.Println(tui.WarnLine(r.name, r.err.Error()))
			fmt.Println()
		} else {
			fmt.Print(r.output)
		}
	}
	return nil
}

func runReposSetup(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Repos) == 0 {
		fmt.Println("no repos configured — add entries under repos: in config.yaml")
		return nil
	}

	home := resolveReposPath(cfg)
	for _, repo := range cfg.Repos {
		if repo.GitProfile == "" {
			continue
		}
		path := repoRoot(home, repo.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("%s: not cloned — skipping\n", repo.Name)
			continue
		}
		msg := applyRepoProfile(path, repo, cfg)
		if msg != "" {
			fmt.Printf("%s: %s\n", repo.Name, msg)
		}
	}
	return nil
}

func repoStatus(home string, repo config.RepoConfig) repoResult {
	path := repoRoot(home, repo.Path)
	var sb strings.Builder

	badge := ""
	if repo.Readonly {
		badge = tui.StyleMuted.Render("readonly")
	}
	fmt.Fprintln(&sb, tui.SectionHeader(repo.Name, badge))

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintln(&sb, "  "+tui.StyleMuted.Render("not cloned — run: overseer repos pull"))
		fmt.Fprintln(&sb)
		return repoResult{name: repo.Name, output: sb.String()}
	}

	out, err := gitIn(path, "status", "--short", "--branch")
	if err != nil {
		return repoResult{name: repo.Name, err: err}
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		fmt.Fprintf(&sb, "  %s\n", colorGitStatusLine(line))
	}
	fmt.Fprintln(&sb)

	return repoResult{name: repo.Name, readonly: repo.Readonly, output: sb.String()}
}

func repoPull(home string, repo config.RepoConfig, cfg *config.Config) repoResult {
	path := repoRoot(home, repo.Path)
	var sb strings.Builder

	badge := ""
	if repo.Readonly {
		badge = tui.StyleMuted.Render("readonly")
	}
	fmt.Fprintln(&sb, tui.SectionHeader(repo.Name, badge))

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(&sb, "  %s\n", tui.StyleMuted.Render("cloning "+repo.URL+"..."))
		out, err := git("clone", repo.URL, path)
		if err != nil {
			return repoResult{name: repo.Name, err: fmt.Errorf("clone failed: %s", out)}
		}
		fmt.Fprintf(&sb, "  %s\n", tui.StyleOK.Render("cloned"))
		if msg := applyRepoProfile(path, repo, cfg); msg != "" {
			fmt.Fprintf(&sb, "  %s\n", tui.StyleDim.Render(msg))
		}
		fmt.Fprintln(&sb)
		return repoResult{name: repo.Name, output: sb.String()}
	}

	out, err := gitIn(path, "pull", "--ff-only")
	if err != nil {
		return repoResult{name: repo.Name, err: fmt.Errorf("pull failed: %s", strings.TrimSpace(out))}
	}
	fmt.Fprintf(&sb, "  %s\n\n", tui.StyleDim.Render(strings.TrimSpace(out)))

	return repoResult{name: repo.Name, readonly: repo.Readonly, output: sb.String()}
}

// colorGitStatusLine applies colour to a single line of `git status --short --branch` output.
func colorGitStatusLine(line string) string {
	if len(line) < 2 {
		return line
	}
	xy := line[:2]
	switch {
	case xy == "##":
		return tui.StyleMuted.Render(line)
	case strings.ContainsAny(xy, "M"):
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(line) // amber — modified
	case strings.ContainsAny(xy, "A"):
		return tui.StyleOK.Render(line) // green — added
	case strings.ContainsAny(xy, "D"):
		return tui.StyleError.Render(line) // red — deleted
	case xy == "??":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(line) // blue — untracked
	default:
		return tui.StyleNormal.Render(line)
	}
}

// applyRepoProfile applies the git_profile configured for a repo.
// Returns a status message (empty if no profile configured).
func applyRepoProfile(path string, repo config.RepoConfig, cfg *config.Config) string {
	if repo.GitProfile == "" || repo.Readonly {
		return ""
	}
	var profile *config.GitProfile
	for i := range cfg.Git.Profiles {
		if cfg.Git.Profiles[i].Name == repo.GitProfile {
			profile = &cfg.Git.Profiles[i]
			break
		}
	}
	if profile == nil {
		return fmt.Sprintf("[warn] git profile %q not found", repo.GitProfile)
	}
	resolved, err := resolveProfile(*profile, cfg.Git.Defaults, cfg.System)
	if err != nil {
		return fmt.Sprintf("[warn] resolving profile %q: %v", repo.GitProfile, err)
	}
	if err := applyGitConfigIn(path, gitScopeLocal, resolved); err != nil {
		return fmt.Sprintf("[warn] applying profile %q: %v", repo.GitProfile, err)
	}
	return fmt.Sprintf("applied git profile %q", repo.GitProfile)
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).CombinedOutput()
	return string(out), err
}

func gitIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
