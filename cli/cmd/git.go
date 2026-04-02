package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/secrets"
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

func init() {
	gitSetupCmd.Flags().BoolVar(&gitSetupGlobal, "global", false, "Apply profile to global git config instead of local repo")
	gitCmd.AddCommand(gitSetupCmd)
	rootCmd.AddCommand(gitCmd)
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
	fmt.Println("select a git profile:")
	for i, p := range profiles {
		fmt.Printf("  %d) %s (%s)\n", i+1, p.Name, p.Email)
	}
	fmt.Print("enter number: ")

	var choice int
	if _, err := fmt.Scan(&choice); err != nil || choice < 1 || choice > len(profiles) {
		return config.GitProfile{}, fmt.Errorf("invalid selection")
	}

	return profiles[choice-1], nil
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
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
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
