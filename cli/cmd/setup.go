package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/symlink"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var dryRun bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard",
	Long: `Walks through all overseer configuration in one interactive session.

Safe to run multiple times — existing values are shown as defaults,
dotfile symlinks are idempotent, and files are backed up before being replaced.`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without making them")
	rootCmd.AddCommand(setupCmd)
}

// brainSource is where setup gets the brain from.
type brainSource int

const (
	brainOnDisk brainSource = iota // already present, cloned or copied earlier
	brainClone                     // cloned from a remote during setup
	brainNew                       // scaffolded from scratch
)

func runSetup(_ *cobra.Command, _ []string) error {
	fmt.Println(tui.Logo(Version))
	fmt.Println()
	if dryRun {
		fmt.Println(tui.StyleWarn.Render("dry run — no changes will be made"))
		fmt.Println()
	}

	// --- Load whatever config exists so we can show current values as defaults.
	existing, _ := config.Load()
	if existing == nil {
		existing = &config.Config{}
	}

	// -------------------------------------------------------------------------
	// Section 1: Brain
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("brain", "where your personal config, dotfiles, and Brewfile live"))
	fmt.Println()

	source, brainPath, brainURL, err := promptBrain(existing)
	if err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Section 2: Machine
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("machine", "settings specific to this machine"))
	fmt.Println()

	currentHome := existing.System.ReposPath
	if currentHome == "" {
		currentHome = defaultReposPath()
	}
	reposPath, err := tui.Prompt("repos_path (where managed repos are cloned)", currentHome, currentHome)
	if err != nil {
		return err
	}
	fmt.Println()

	currentGPG := existing.System.GPGSSHProgram
	if currentGPG == "" {
		currentGPG = defaultGPGSSHProgram()
	}
	gpgSSHProgram, err := tui.Prompt("gpg_ssh_program (SSH signing binary, leave blank to skip)", currentGPG, currentGPG)
	if err != nil {
		return err
	}
	fmt.Println()

	// -------------------------------------------------------------------------
	// Write config.local.yaml
	// -------------------------------------------------------------------------
	localPath, err := config.LocalPath()
	if err != nil {
		return err
	}

	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("creating config dir: %w", err)
		}
		content := buildLocalConfig(brainPath, reposPath, gpgSSHProgram)
		if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing config.local.yaml: %w", err)
		}
	}
	fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("wrote "+localPath))
	fmt.Println()

	// -------------------------------------------------------------------------
	// Section 3: Clone — before anything that reads the brain's config
	// -------------------------------------------------------------------------
	if source == brainClone {
		fmt.Println(tui.SectionHeader("brain", "clone "+brainURL))
		fmt.Println()
		if dryRun {
			fmt.Printf("  %s\n", tui.StyleMuted.Render("would clone into "+brainPath))
		} else if err := cloneBrain(brainURL, brainPath); err != nil {
			return err
		}
		fmt.Println()
	}

	// Reload now that local config is written and the brain is in place, so
	// profiles, overlays and brain settings come from the real brain.
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	overseerDir := config.BrainOverseerPath(cfg)
	warnSharedSigner(cfg)

	// -------------------------------------------------------------------------
	// Section 4: Brain scaffold (new brains only)
	// -------------------------------------------------------------------------
	if source == brainNew {
		fmt.Println(tui.SectionHeader("brain", "scaffold directory structure"))
		fmt.Println()
		if err := scaffoldBrain(overseerDir); err != nil {
			return err
		}
		if !dryRun && brainURL != "" && brainURL != existing.Brain.URL {
			if err := setBrainConfigValue(overseerDir, "url", brainURL); err != nil {
				fmt.Println(tui.WarnLine("brain", "could not write url to config.yaml: "+err.Error()))
			}
		}
		fmt.Println()
	}

	// -------------------------------------------------------------------------
	// Section 5: Git defaults + initial profile (skipped if the brain has profiles)
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("git", "identity profiles"))
	fmt.Println()

	if len(cfg.Git.Profiles) > 0 {
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render("[skip]"), tui.StyleMuted.Render(fmt.Sprintf("%d profile(s) already configured", len(cfg.Git.Profiles))))
	} else if err := promptInitialGitConfig(cfg, filepath.Join(overseerDir, "config.yaml")); err != nil {
		return err
	}
	fmt.Println()

	// -------------------------------------------------------------------------
	// Section 6: Brain version control
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("brain", "version control"))
	fmt.Println()

	if dryRun {
		fmt.Printf("  %s\n", tui.StyleMuted.Render("skipped in dry-run mode"))
	} else if err := runSetupBrainGit(cfg, brainURL); err != nil {
		fmt.Println(tui.WarnLine("git", err.Error()))
	}

	fmt.Println()

	// -------------------------------------------------------------------------
	// Section 7: Dotfiles
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("dotfiles", "wire from brain into ~/"))
	fmt.Println()

	if err := wireDotfiles(cfg, dryRun, true); err != nil {
		return err
	}

	fmt.Println()

	// -------------------------------------------------------------------------
	// Section 8: Packages (macOS only)
	// -------------------------------------------------------------------------
	if runtime.GOOS == "darwin" && brewAvailable() {
		fmt.Println(tui.SectionHeader("packages", "install Brewfile packages"))
		fmt.Println()
		if err := runBrewInstall(nil, nil); err != nil {
			fmt.Println(tui.WarnLine("brew", err.Error()))
		}
		fmt.Println()
	}

	// -------------------------------------------------------------------------
	// Done
	// -------------------------------------------------------------------------
	fmt.Println(tui.StyleOK.Render("✓") + "  " + tui.StyleNormal.Render("setup complete"))
	fmt.Println()
	if source == brainNew {
		fmt.Println(tui.StyleMuted.Render("Next: edit " + filepath.Join(overseerDir, "config.yaml") + " to add integrations and git profiles."))
	} else {
		fmt.Println(tui.StyleMuted.Render("Next: overseer brain status — checks for paths that only work on another platform."))
	}

	return nil
}

// promptBrain decides where the brain comes from. A brain already at the
// resolved path is used without asking; otherwise the user picks between
// cloning, starting fresh, or pointing at a brain elsewhere on disk.
func promptBrain(existing *config.Config) (brainSource, string, string, error) {
	currentBrain := config.ResolveBrainPath(existing)
	if _, err := os.Stat(filepath.Join(currentBrain, "overseer", "config.yaml")); err == nil {
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("using brain at "+currentBrain))
		fmt.Println()
		return brainOnDisk, currentBrain, existing.Brain.URL, nil
	}

	idx, err := tui.Select("no brain found at "+currentBrain+" — where does it come from?", []tui.SelectItem{
		{Title: "clone existing", Subtitle: "you already have a brain repository — clone it here"},
		{Title: "start new", Subtitle: "scaffold a fresh brain, optionally under git"},
		{Title: "already on disk", Subtitle: "a brain exists in another directory on this machine"},
	})
	if err != nil {
		return 0, "", "", err
	}
	if idx == -1 {
		return 0, "", "", fmt.Errorf("setup cancelled")
	}
	fmt.Println()
	source := []brainSource{brainClone, brainNew, brainOnDisk}[idx]

	brainPath, err := tui.Prompt("brain_path", currentBrain, currentBrain)
	if err != nil {
		return 0, "", "", err
	}
	brainPath = config.ExpandPath(brainPath)
	fmt.Println()

	switch source {
	case brainOnDisk:
		if _, err := os.Stat(filepath.Join(brainPath, "overseer", "config.yaml")); err != nil {
			return 0, "", "", fmt.Errorf("no overseer/config.yaml under %s", brainPath)
		}
		return source, brainPath, existing.Brain.URL, nil
	case brainClone:
		brainURL, err := tui.Prompt("brain remote URL", existing.Brain.URL, "")
		if err != nil {
			return 0, "", "", err
		}
		if brainURL == "" {
			return 0, "", "", fmt.Errorf("a remote URL is required to clone a brain")
		}
		fmt.Println()
		return source, brainPath, brainURL, nil
	}

	brainURL, err := tui.Prompt("brain remote URL (leave blank to skip)", existing.Brain.URL, existing.Brain.URL)
	if err != nil {
		return 0, "", "", err
	}
	fmt.Println()
	return source, brainPath, brainURL, nil
}

// scaffoldBrain creates the brain directory layout and starter files,
// skipping anything that already exists.
func scaffoldBrain(overseerDir string) error {
	dirs := []struct{ path, label string }{
		{filepath.Join(overseerDir, "dotfiles"), "overseer/dotfiles/"},
		{filepath.Join(overseerDir, "plugins"), "overseer/plugins/"},
	}
	for _, d := range dirs {
		if _, err := os.Stat(d.path); err == nil {
			fmt.Printf("  %s  %s\n", tui.StyleDim.Render("[skip]"), tui.StyleMuted.Render(d.label+" already exists"))
			continue
		}
		if !dryRun {
			if err := os.MkdirAll(d.path, 0755); err != nil {
				return fmt.Errorf("creating %s: %w", d.path, err)
			}
		}
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("[mkdir]"), tui.StyleNormal.Render(d.label))
	}

	for _, file := range []struct {
		path, content, label string
	}{
		{
			filepath.Join(overseerDir, "Brewfile"),
			"# Add your Homebrew packages here\n",
			"overseer/Brewfile",
		},
		{
			filepath.Join(overseerDir, "Brewfile.local.example"),
			"# Machine-specific packages — copy to Brewfile.local (gitignored)\n",
			"overseer/Brewfile.local.example",
		},
		{
			filepath.Join(overseerDir, "config.yaml"),
			brainConfigExample,
			"overseer/config.yaml",
		},
	} {
		if _, err := os.Stat(file.path); err == nil {
			fmt.Printf("  %s  %s\n", tui.StyleDim.Render("[skip]"), tui.StyleMuted.Render(file.label+" already exists"))
			continue
		}
		if !dryRun {
			if err := os.WriteFile(file.path, []byte(file.content), 0644); err != nil {
				return fmt.Errorf("creating %s: %w", file.path, err)
			}
		}
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("[create]"), tui.StyleNormal.Render(file.label))
	}
	return nil
}

// promptInitialGitConfig offers to create git defaults and a first profile and
// writes them to the brain config.
func promptInitialGitConfig(cfg *config.Config, cfgPath string) error {
	wantsDefaults, err := tui.Confirm("configure git defaults (user name, gpg format, signing)?")
	if err != nil {
		return err
	}
	fmt.Println()

	var gitDefaults *config.GitDefaults
	var defaults config.GitDefaults
	if wantsDefaults {
		defaults, err = promptDefaults(cfg.Git.Defaults)
		if err != nil {
			return err
		}
		gitDefaults = &defaults
	}

	wantsProfile, err := tui.Confirm("create an initial git profile?")
	if err != nil {
		return err
	}
	fmt.Println()

	var initialProfile *config.GitProfile
	if wantsProfile {
		p, err := promptProfile(config.GitProfile{}, defaults)
		if err != nil {
			return err
		}
		initialProfile = &p
	}

	if dryRun || (gitDefaults == nil && initialProfile == nil) {
		return nil
	}
	if err := writeGitConfig(cfgPath, gitDefaults, initialProfile); err != nil {
		fmt.Println(tui.WarnLine("git", "could not write git config to config.yaml: "+err.Error()))
		return nil
	}
	if gitDefaults != nil {
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("git defaults saved"))
	}
	if initialProfile != nil {
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("profile "+initialProfile.Name+" created"))
	}
	return nil
}

// warnSharedSigner flags a signer set in the brain that overrides this
// machine's system.gpg_ssh_program, a common breakage when a brain made on
// macOS is cloned on Linux or WSL.
func warnSharedSigner(cfg *config.Config) {
	shared := cfg.Git.Defaults.GPGSSHProgram
	if shared == "" || shared == cfg.System.GPGSSHProgram {
		return
	}
	if _, err := os.Stat(shared); err == nil {
		return
	}
	fmt.Println(tui.WarnLine("git", "git.defaults.gpg_ssh_program ("+shared+") doesn't exist here and overrides system.gpg_ssh_program — move it to config."+config.Platform()+".yaml in the brain"))
	fmt.Println()
}

// wireDotfiles links dotfiles/ and the dotfiles.<platform> overlays into ~/,
// and removes links for brain files that don't apply on this platform. With
// confirm, files that would be replaced are listed first and the user can
// keep them.
func wireDotfiles(cfg *config.Config, dry, confirm bool) error {
	active, err := symlink.Collect(config.DotfilesDirs(cfg))
	if err != nil {
		return fmt.Errorf("reading dotfiles: %w", err)
	}
	known, err := symlink.Collect(config.AllDotfilesDirs(cfg))
	if err != nil {
		return fmt.Errorf("reading dotfiles: %w", err)
	}
	overseerDir := config.BrainOverseerPath(cfg)
	if len(known) == 0 {
		fmt.Printf("  %s\n", tui.StyleMuted.Render("no dotfiles found — add files under "+filepath.Join(overseerDir, "dotfiles")+"/"))
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	if conflicts := symlink.Conflicts(active, home); confirm && !dry && len(conflicts) > 0 {
		fmt.Printf("  %s\n", tui.StyleNormal.Render("these files in ~/ would be replaced by links into the brain:"))
		for _, rel := range conflicts {
			fmt.Printf("     %s  %s\n", tui.StyleMuted.Render("·"), tui.StyleNormal.Render(rel+"  ← "+symlinkLayer(overseerDir, active[rel])))
		}
		replace, err := tui.Confirm("replace them? (originals are moved to ~/.overseer-backups/)")
		if err != nil {
			return err
		}
		fmt.Println()
		if !replace {
			for _, rel := range conflicts {
				delete(active, rel)
			}
			fmt.Printf("  %s\n", tui.StyleMuted.Render("kept your files — a brain file that only fits one platform belongs in dotfiles.<platform>/"))
		}
	}

	if err := symlink.LinkAll(active, overseerDir, home, dry); err != nil {
		return fmt.Errorf("wiring dotfiles: %w", err)
	}
	if err := symlink.Prune(known, active, overseerDir, home, dry); err != nil {
		return fmt.Errorf("removing stale dotfile links: %w", err)
	}
	return nil
}

// symlinkLayer names the brain layer a dotfile comes from, e.g. "dotfiles.wsl/".
func symlinkLayer(overseerDir, source string) string {
	rel, err := filepath.Rel(overseerDir, source)
	if err != nil {
		return source
	}
	return strings.SplitN(filepath.ToSlash(rel), "/", 2)[0] + "/"
}

// runBrainSetup wires dotfiles from brain and installs Brew packages.
// Shared by overseer brain setup.
func runBrainSetup(cfg *config.Config, dry bool) error {
	if dry {
		fmt.Println("overseer setup (dry run)")
	} else {
		fmt.Println("overseer setup")
	}
	fmt.Printf("  brain: %s\n\n", config.ResolveBrainPath(cfg))

	if err := wireDotfiles(cfg, dry, false); err != nil {
		return err
	}

	if runtime.GOOS == "darwin" && brewAvailable() {
		fmt.Println()
		if err := runBrewInstall(nil, nil); err != nil {
			fmt.Println(tui.WarnLine("brew", err.Error()))
		}
	}

	fmt.Println("\nDone.")
	return nil
}

// runSetupBrainGit is called from the setup wizard. A brain that is already a
// repository gets brain.git_profile applied; otherwise the user may turn it
// into one.
func runSetupBrainGit(cfg *config.Config, configuredURL string) error {
	brainPath := config.ResolveBrainPath(cfg)

	if brainIsGit(brainPath) {
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("git repository"))
		applyBrainGitProfile(brainPath, cfg)
		return nil
	}

	confirmed, err := tui.Confirm("set up brain as a git repository?")
	if err != nil || !confirmed {
		fmt.Printf("  %s\n", tui.StyleMuted.Render("skipped — brain will remain a plain folder"))
		return nil
	}
	fmt.Println()
	return initBrainRepo(cfg, configuredURL)
}

func writeGitConfig(cfgPath string, defaults *config.GitDefaults, profile *config.GitProfile) error {
	doc, err := config.ReadConfigNode(cfgPath)
	if err != nil {
		return err
	}
	if defaults != nil {
		config.SetDefaults(doc, *defaults)
	}
	if profile != nil {
		config.UpsertProfile(doc, *profile)
	}
	return config.WriteConfigNode(cfgPath, doc)
}

// setBrainConfigValue appends or updates a top-level key under `brain:` in
// the brain's overseer/config.yaml. It reads the raw file and does a simple
// string manipulation so it never loses existing content or comments.
func setBrainConfigValue(overseerDir, key, value string) error {
	cfgPath := filepath.Join(overseerDir, "config.yaml")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		// File doesn't exist yet — nothing to update; caller will create it.
		return nil
	}

	lines := strings.Split(string(data), "\n")

	// Find an existing `  key: ...` line inside the brain: block.
	inBrain := false
	keyLine := -1
	brainLine := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "brain:" {
			inBrain = true
			brainLine = i
			continue
		}
		if inBrain {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Exited brain block (new top-level key).
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				inBrain = false
				continue
			}
			if strings.HasPrefix(trimmed, key+":") {
				keyLine = i
			}
		}
	}

	entry := fmt.Sprintf("  %s: %s", key, value)

	if keyLine >= 0 {
		lines[keyLine] = entry
	} else if brainLine >= 0 {
		// Insert after the brain: line.
		lines = append(lines[:brainLine+1], append([]string{entry}, lines[brainLine+1:]...)...)
	} else {
		// No brain: block — append one.
		lines = append(lines, "", "brain:", entry)
	}

	return os.WriteFile(cfgPath, []byte(strings.Join(lines, "\n")), 0644)
}
