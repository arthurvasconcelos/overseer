package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage git identity profiles",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured git profiles",
	RunE:  runProfileList,
}

var profileAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a new git profile",
	RunE:  runProfileAdd,
}

var profileEditCmd = &cobra.Command{
	Use:   "edit [name]",
	Short: "Edit an existing git profile",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runProfileEdit,
}

var profileRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a git profile",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runProfileRemove,
}

var profileApplyCmd = &cobra.Command{
	Use:   "apply [name]",
	Short: "Apply a git profile to the current repo or globally",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runProfileApply,
}

var profileDefaultsCmd = &cobra.Command{
	Use:   "defaults",
	Short: "View and edit git.defaults",
	RunE:  runProfileDefaults,
}

var profileApplyGlobal bool

func init() {
	profileApplyCmd.Flags().BoolVar(&profileApplyGlobal, "global", false, "Apply to global ~/.gitconfig instead of local repo")
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileEditCmd)
	profileCmd.AddCommand(profileRemoveCmd)
	profileCmd.AddCommand(profileApplyCmd)
	profileCmd.AddCommand(profileDefaultsCmd)
}

// --- list ---

func runProfileList(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Git.Profiles) == 0 {
		fmt.Println(tui.StyleMuted.Render("no git profiles configured"))
		fmt.Println(tui.StyleMuted.Render("run: overseer git profile add"))
		return nil
	}

	fmt.Println(tui.SectionHeader("git profiles", ""))
	fmt.Println()

	globalEmail, _ := gitConfigGet("--global", "user.email")

	for _, p := range cfg.Git.Profiles {
		name := tui.StyleAccent.Render(p.Name)
		email := tui.StyleNormal.Render(p.Email)

		badge := ""
		if p.Email != "" && strings.TrimSpace(globalEmail) == p.Email {
			badge = tui.StyleOK.Render("[global]")
		}

		fmt.Printf("  %s  %s", name, email)
		if badge != "" {
			fmt.Printf("  %s", badge)
		}
		fmt.Println()

		if p.UserName != "" {
			fmt.Printf("     %s %s\n", tui.StyleDim.Render("name:"), tui.StyleNormal.Render(p.UserName))
		}
		gpgFmt := coalesce(p.GPGFormat, cfg.Git.Defaults.GPGFormat)
		if gpgFmt != "" {
			fmt.Printf("     %s %s\n", tui.StyleDim.Render("gpg:"), tui.StyleMuted.Render(gpgFmt))
		}
		if p.SigningKey != "" {
			fmt.Printf("     %s %s\n", tui.StyleDim.Render("key:"), tui.StyleMuted.Render(p.SigningKey))
		}
		fmt.Println()
	}

	return nil
}

// --- add ---

func runProfileAdd(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	p, err := promptProfile(GitProfile{}, cfg.Git.Defaults)
	if err != nil {
		return err
	}

	// Name must be unique.
	for _, existing := range cfg.Git.Profiles {
		if existing.Name == p.Name {
			return fmt.Errorf("a profile named %q already exists — use: overseer git profile edit %s", p.Name, p.Name)
		}
	}

	if err := saveProfile(cfg, p); err != nil {
		return err
	}

	fmt.Printf("%s  profile %s created\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(p.Name))
	return nil
}

// --- edit ---

func runProfileEdit(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Git.Profiles) == 0 {
		return fmt.Errorf("no git profiles configured — run: overseer git profile add")
	}

	profile, err := pickOrArgProfile(cfg.Git.Profiles, args, "select a profile to edit")
	if err != nil {
		return err
	}

	updated, err := promptProfile(profile, cfg.Git.Defaults)
	if err != nil {
		return err
	}
	// Preserve the original name — editing doesn't rename.
	updated.Name = profile.Name

	if err := saveProfile(cfg, updated); err != nil {
		return err
	}

	fmt.Printf("%s  profile %s updated\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(updated.Name))
	return nil
}

// --- remove ---

func runProfileRemove(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Git.Profiles) == 0 {
		return fmt.Errorf("no git profiles configured")
	}

	profile, err := pickOrArgProfile(cfg.Git.Profiles, args, "select a profile to remove")
	if err != nil {
		return err
	}

	// Safety: check for references.
	var refs []string
	if cfg.Brain.GitProfile == profile.Name {
		refs = append(refs, "brain.git_profile")
	}
	for _, r := range cfg.Repos {
		if r.GitProfile == profile.Name {
			refs = append(refs, fmt.Sprintf("repos[%s].git_profile", r.Name))
		}
	}
	if len(refs) > 0 {
		fmt.Printf("%s  profile %s is referenced by:\n", tui.StyleWarn.Render("⚠"), tui.StyleAccent.Render(profile.Name))
		for _, ref := range refs {
			fmt.Printf("   %s\n", tui.StyleMuted.Render("· "+ref))
		}
		fmt.Println()
	}

	confirmed, err := tui.Confirm(fmt.Sprintf("remove profile %q?", profile.Name))
	if err != nil || !confirmed {
		fmt.Println(tui.StyleMuted.Render("cancelled"))
		return nil
	}

	cfgPath, err := brainConfigPath(cfg)
	if err != nil {
		return err
	}
	doc, err := config.ReadConfigNode(cfgPath)
	if err != nil {
		return err
	}
	if !config.DeleteProfile(doc, profile.Name) {
		return fmt.Errorf("profile %q not found in config", profile.Name)
	}
	if err := config.WriteConfigNode(cfgPath, doc); err != nil {
		return err
	}

	fmt.Printf("%s  profile %s removed\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(profile.Name))
	return nil
}

// --- apply ---

func runProfileApply(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Git.Profiles) == 0 {
		return fmt.Errorf("no git profiles configured — run: overseer git profile add")
	}

	profile, err := pickOrArgProfile(cfg.Git.Profiles, args, "select a git profile")
	if err != nil {
		return err
	}

	resolved, err := resolveProfile(profile, cfg.Git.Defaults, cfg.System)
	if err != nil {
		return err
	}

	if profileApplyGlobal {
		if err := applyGitConfig(gitScopeGlobal, resolved); err != nil {
			return err
		}
		fmt.Printf("%s  applied %s globally\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(profile.Name))
		return nil
	}

	// Local: guard against non-git directories.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if !isGitRepo(cwd) {
		return fmt.Errorf("not a git repository: %s", cwd)
	}

	// Warn if a local identity is already set.
	existingEmail, _ := gitConfigGet("--local", "user.email")
	existingEmail = strings.TrimSpace(existingEmail)
	if existingEmail != "" && existingEmail != resolved.Email {
		fmt.Printf("%s  this repo already has user.email = %s\n",
			tui.StyleWarn.Render("⚠"), tui.StyleMuted.Render(existingEmail))
		fmt.Println()
		confirmed, err := tui.Confirm("override with profile " + profile.Name + "?")
		if err != nil || !confirmed {
			fmt.Println(tui.StyleMuted.Render("cancelled"))
			return nil
		}
	}

	if err := applyGitConfig(gitScopeLocal, resolved); err != nil {
		return err
	}
	fmt.Printf("%s  applied %s to %s\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(profile.Name), tui.StyleMuted.Render(filepath.Base(cwd)))
	return nil
}

// --- defaults ---

func runProfileDefaults(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println(tui.SectionHeader("git defaults", "inherited by all profiles"))
	fmt.Println()

	d := cfg.Git.Defaults
	updated, err := promptDefaults(d)
	if err != nil {
		return err
	}

	cfgPath, err := brainConfigPath(cfg)
	if err != nil {
		return err
	}
	doc, err := config.ReadConfigNode(cfgPath)
	if err != nil {
		return err
	}
	config.SetDefaults(doc, updated)
	if err := config.WriteConfigNode(cfgPath, doc); err != nil {
		return err
	}

	fmt.Printf("\n%s  git defaults saved\n", tui.StyleOK.Render("✓"))
	return nil
}

// --- shared helpers ---

// promptProfile shows interactive prompts to fill or edit a GitProfile.
// current holds existing values shown as defaults.
func promptProfile(current GitProfile, defaults config.GitDefaults) (GitProfile, error) {
	name, err := tui.Prompt("profile name", current.Name, current.Name)
	if err != nil {
		return GitProfile{}, err
	}
	fmt.Println()

	email, err := tui.Prompt("email", current.Email, current.Email)
	if err != nil {
		return GitProfile{}, err
	}
	fmt.Println()

	defaultName := coalesce(current.UserName, defaults.UserName)
	userName, err := tui.Prompt("user name (leave blank to inherit from defaults)", current.UserName, defaultName)
	if err != nil {
		return GitProfile{}, err
	}
	if userName == defaultName && current.UserName == "" {
		userName = "" // don't store what's already in defaults
	}
	fmt.Println()

	signingKey, err := tui.Prompt("signing key or op:// reference (leave blank to skip)", current.SigningKey, current.SigningKey)
	if err != nil {
		return GitProfile{}, err
	}
	fmt.Println()

	gpgFormat, err := tui.Prompt("gpg format (ssh / openpgp, leave blank to inherit)", current.GPGFormat, coalesce(current.GPGFormat, defaults.GPGFormat))
	if err != nil {
		return GitProfile{}, err
	}
	if gpgFormat == defaults.GPGFormat && current.GPGFormat == "" {
		gpgFormat = ""
	}
	fmt.Println()

	opAccount, err := tui.Prompt("1password account ID for op:// references (leave blank to skip)", current.OPAccount, current.OPAccount)
	if err != nil {
		return GitProfile{}, err
	}
	fmt.Println()

	return GitProfile{
		Name:      name,
		Email:     email,
		UserName:  userName,
		SigningKey: signingKey,
		GPGFormat: gpgFormat,
		OPAccount: opAccount,
	}, nil
}

// promptDefaults shows interactive prompts to fill or edit GitDefaults.
func promptDefaults(current config.GitDefaults) (config.GitDefaults, error) {
	userName, err := tui.Prompt("user name (shared across profiles)", current.UserName, current.UserName)
	if err != nil {
		return config.GitDefaults{}, err
	}
	fmt.Println()

	gpgFormat, err := tui.Prompt("gpg format (ssh / openpgp)", current.GPGFormat, current.GPGFormat)
	if err != nil {
		return config.GitDefaults{}, err
	}
	fmt.Println()

	gpgSSHProgram, err := tui.Prompt("gpg ssh program path (leave blank to skip)", current.GPGSSHProgram, current.GPGSSHProgram)
	if err != nil {
		return config.GitDefaults{}, err
	}
	fmt.Println()

	commitGPGSign, err := tui.Confirm("enable commit.gpgsign by default?")
	if err != nil {
		return config.GitDefaults{}, err
	}
	fmt.Println()

	return config.GitDefaults{
		UserName:      userName,
		GPGFormat:     gpgFormat,
		GPGSSHProgram: gpgSSHProgram,
		CommitGPGSign: commitGPGSign,
	}, nil
}

// saveProfile writes a profile to the brain config.yaml via yaml.Node.
func saveProfile(cfg *config.Config, p GitProfile) error {
	cfgPath, err := brainConfigPath(cfg)
	if err != nil {
		return err
	}
	doc, err := config.ReadConfigNode(cfgPath)
	if err != nil {
		return err
	}
	config.UpsertProfile(doc, p)
	return config.WriteConfigNode(cfgPath, doc)
}

// brainConfigPath returns the path to brain/overseer/config.yaml.
func brainConfigPath(cfg *config.Config) (string, error) {
	p := filepath.Join(config.BrainOverseerPath(cfg), "config.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return "", fmt.Errorf("creating brain overseer dir: %w", err)
	}
	return p, nil
}

// pickOrArgProfile resolves the target profile from CLI args or a TUI picker.
func pickOrArgProfile(profiles []config.GitProfile, args []string, title string) (config.GitProfile, error) {
	if len(args) > 0 {
		name := args[0]
		for _, p := range profiles {
			if p.Name == name {
				return p, nil
			}
		}
		return config.GitProfile{}, fmt.Errorf("no profile named %q", name)
	}
	return pickProfile(profiles)
}

// isGitRepo returns true if dir is inside a git repository.
func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// gitConfigGet reads a single git config value at the given scope flag (e.g. "--global").
// Returns empty string and no error if the key is unset.
func gitConfigGet(scopeFlag, key string) (string, error) {
	cmd := exec.Command("git", "config", scopeFlag, key)
	out, err := cmd.Output()
	if err != nil {
		return "", nil // unset is not an error
	}
	return strings.TrimSpace(string(out)), nil
}

// GitProfile is a local alias so this file can reference the config type directly.
type GitProfile = config.GitProfile
