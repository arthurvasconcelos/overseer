package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Git utilities",
}

var gitSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Apply a git identity profile to the current repo (or globally)",
	RunE:  runGitSetup,
}

const (
	gitScopeLocal  = "local"
	gitScopeGlobal = "global"
)

var gitSetupGlobal bool

var gitBranchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Show the active git identity for the current repo and branch",
	RunE:  runGitBranch,
}

func init() {
	gitSetupCmd.Flags().BoolVar(&gitSetupGlobal, "global", false, "Apply profile to global git config instead of local repo")
	gitCmd.AddCommand(gitSetupCmd)
	gitCmd.AddCommand(gitBranchCmd)
	gitCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(gitCmd)
}

func runGitBranch(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if !isGitRepo(cwd) {
		return fmt.Errorf("not a git repository: %s", cwd)
	}

	branch, err := gitIn(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("reading branch: %w", err)
	}
	branch = strings.TrimSpace(branch)

	localEmail, _ := gitConfigGetIn(cwd, "--local", "user.email")
	localName, _ := gitConfigGetIn(cwd, "--local", "user.name")
	localSignKey, _ := gitConfigGetIn(cwd, "--local", "user.signingkey")
	localGPGSign, _ := gitConfigGetIn(cwd, "--local", "commit.gpgsign")
	localGPGFmt, _ := gitConfigGetIn(cwd, "--local", "gpg.format")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var matchedProfile string
	for _, p := range cfg.Git.Profiles {
		if p.Email != "" && p.Email == localEmail {
			matchedProfile = p.Name
			break
		}
	}

	type branchJSON struct {
		Branch     string `json:"branch"`
		Profile    string `json:"profile,omitempty"`
		Email      string `json:"email,omitempty"`
		Name       string `json:"name,omitempty"`
		GPGSign    bool   `json:"gpg_sign"`
		GPGFormat  string `json:"gpg_format,omitempty"`
		SigningKey  string `json:"signing_key,omitempty"`
	}

	if outputFormat == "json" {
		return printJSON(branchJSON{
			Branch:    branch,
			Profile:   matchedProfile,
			Email:     localEmail,
			Name:      localName,
			GPGSign:   localGPGSign == "true",
			GPGFormat: localGPGFmt,
			SigningKey: localSignKey,
		})
	}

	fmt.Println(tui.SectionHeader("git branch", branch))
	fmt.Println()

	profileLabel := tui.StyleMuted.Render("(no match)")
	if matchedProfile != "" {
		profileLabel = tui.StyleAccent.Render(matchedProfile)
	}
	fmt.Printf("  %s  %s\n", tui.StyleDim.Render("profile   "), profileLabel)

	if localEmail != "" {
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render("email     "), tui.StyleNormal.Render(localEmail))
	}
	if localName != "" {
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render("name      "), tui.StyleNormal.Render(localName))
	}

	gpgSign := tui.StyleMuted.Render("off")
	if localGPGSign == "true" {
		gpgFmt := localGPGFmt
		if gpgFmt == "" {
			gpgFmt = coalesce(cfg.Git.Defaults.GPGFormat, "openpgp")
		}
		gpgSign = tui.StyleOK.Render("✓") + " " + tui.StyleMuted.Render(gpgFmt)
	}
	fmt.Printf("  %s  %s\n", tui.StyleDim.Render("gpg sign  "), gpgSign)

	if localSignKey != "" {
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render("signing key"), tui.StyleMuted.Render(localSignKey))
	}

	fmt.Println()
	return nil
}

func runGitSetup(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Git.Profiles) == 0 {
		return fmt.Errorf("no git profiles configured — add profiles under git.profiles in %s", "~/.config/overseer/config.yaml")
	}

	profile, err := pickProfile(cfg.Git.Profiles)
	if err != nil {
		return err
	}

	resolved, err := resolveProfile(profile, cfg.Git.Defaults, cfg.System)
	if err != nil {
		return err
	}

	scope := gitScopeLocal
	if gitSetupGlobal {
		scope = gitScopeGlobal
	}

	if err := applyGitConfig(scope, resolved); err != nil {
		return err
	}

	fmt.Printf("applied git profile \"%s\" (%s)\n", profile.Name, scope)
	return nil
}

func pickProfile(profiles []config.GitProfile) (config.GitProfile, error) {
	items := make([]tui.SelectItem, len(profiles))
	for i, p := range profiles {
		items[i] = tui.SelectItem{Title: p.Name, Subtitle: p.Email}
	}

	idx, err := tui.Select("select a git profile", items)
	if err != nil {
		return config.GitProfile{}, err
	}
	if idx == -1 {
		return config.GitProfile{}, fmt.Errorf("cancelled")
	}
	return profiles[idx], nil
}

// resolvedProfile holds the final merged values ready to pass to git config.
type resolvedProfile struct {
	UserName      string
	Email         string
	SigningKey     string
	GPGFormat     string
	GPGSSHProgram string
	CommitGPGSign bool
}

func resolveProfile(p config.GitProfile, d config.GitDefaults, sys config.SystemConfig) (resolvedProfile, error) {
	r := resolvedProfile{
		UserName:      coalesce(p.UserName, d.UserName),
		Email:         p.Email,
		GPGFormat:     coalesce(p.GPGFormat, d.GPGFormat),
		GPGSSHProgram: coalesce(p.GPGSSHProgram, d.GPGSSHProgram, sys.GPGSSHProgram),
		CommitGPGSign: d.CommitGPGSign,
	}
	if p.CommitGPGSign != nil {
		r.CommitGPGSign = *p.CommitGPGSign
	}

	// Resolve signing key — plain value or op:// reference.
	signingKey := p.SigningKey
	if strings.HasPrefix(signingKey, "op://") {
		resolved, err := secrets.ReadAs(signingKey, p.OPAccount)
		if err != nil {
			return r, fmt.Errorf("resolving signing_key: %w", err)
		}
		signingKey = resolved
	}
	r.SigningKey = signingKey

	return r, nil
}

func applyGitConfig(scope string, r resolvedProfile) error {
	return applyGitConfigIn("", scope, r)
}

func applyGitConfigIn(dir, scope string, r resolvedProfile) error {
	settings := [][]string{
		{"user.name", r.UserName},
		{"user.email", r.Email},
		{"gpg.format", r.GPGFormat},
		{"commit.gpgsign", fmt.Sprintf("%t", r.CommitGPGSign)},
	}
	if r.SigningKey != "" {
		settings = append(settings, []string{"user.signingkey", r.SigningKey})
	}
	if r.GPGSSHProgram != "" {
		settings = append(settings, []string{"gpg.ssh.program", r.GPGSSHProgram})
	}

	for _, s := range settings {
		args := []string{"config", "--" + scope, s[0], s[1]}
		cmd := exec.Command("git", args...)
		if dir != "" {
			cmd.Dir = dir
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git config %s: %s", s[0], strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
