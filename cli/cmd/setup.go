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

	currentBrain := config.ResolveBrainPath(existing)
	brainPath, err := tui.Prompt("brain_path", currentBrain, currentBrain)
	if err != nil {
		return err
	}
	fmt.Println()

	currentBrainURL := existing.Brain.URL
	brainURL, err := tui.Prompt("brain remote URL (leave blank to skip)", currentBrainURL, currentBrainURL)
	if err != nil {
		return err
	}
	fmt.Println()

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
	// Section 3: Git defaults + initial profile (skipped if profiles exist)
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("git", "identity profiles"))
	fmt.Println()

	if len(existing.Git.Profiles) > 0 {
		fmt.Printf("  %s  %s\n", tui.StyleDim.Render("[skip]"), tui.StyleMuted.Render(fmt.Sprintf("%d profile(s) already configured", len(existing.Git.Profiles))))
		fmt.Println()
	} else {
		wantsDefaults, err := tui.Confirm("configure git defaults (user name, gpg format, signing)?")
		if err != nil {
			return err
		}
		fmt.Println()

		var gitDefaults config.GitDefaults
		if wantsDefaults {
			gitDefaults, err = promptDefaults(existing.Git.Defaults)
			if err != nil {
				return err
			}
		}

		wantsProfile, err := tui.Confirm("create an initial git profile?")
		if err != nil {
			return err
		}
		fmt.Println()

		var initialProfile *config.GitProfile
		if wantsProfile {
			p, err := promptProfile(config.GitProfile{}, gitDefaults)
			if err != nil {
				return err
			}
			initialProfile = &p
		}

		// Write defaults and profile into brain config now so later wizard
		// steps (e.g. brain git-init) can reference them.
		if wantsDefaults || initialProfile != nil {
			brainOverseerDir := config.BrainOverseerPath(existing)
			cfgPath := filepath.Join(brainOverseerDir, "config.yaml")

			// Brain dir may not exist yet — that's OK, it's created in Section 4.
			// Only write if the dir exists; otherwise defer until brain scaffold runs.
			if _, statErr := os.Stat(brainOverseerDir); statErr == nil {
				doc, readErr := config.ReadConfigNode(cfgPath)
				if readErr == nil {
					if wantsDefaults {
						config.SetDefaults(doc, gitDefaults)
					}
					if initialProfile != nil {
						config.UpsertProfile(doc, *initialProfile)
					}
					_ = config.WriteConfigNode(cfgPath, doc)
				}
			}

			if wantsDefaults {
				fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("git defaults saved"))
			}
			if initialProfile != nil {
				fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("profile "+initialProfile.Name+" created"))
			}
			fmt.Println()
		}
	}

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

	// Reload config now that local is written, so brain paths resolve correctly.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Section 3: Brain scaffold
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("brain", "scaffold directory structure"))
	fmt.Println()

	overseerDir := config.BrainOverseerPath(cfg)

	if !dryRun {
		// Apply brainURL to brain config.yaml if provided.
		if brainURL != "" && brainURL != existing.Brain.URL {
			if err := setBrainConfigValue(overseerDir, "url", brainURL); err != nil {
				fmt.Println(tui.WarnLine("brain", "could not write url to config.yaml: "+err.Error()))
			}
		}
	}

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

	fmt.Println()

	// -------------------------------------------------------------------------
	// Section 4: Brain version control (optional)
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("brain", "version control (optional)"))
	fmt.Println()

	if !dryRun {
		if err := runSetupBrainGit(cfg, brainURL); err != nil {
			fmt.Println(tui.WarnLine("git", err.Error()))
		}
	} else {
		fmt.Printf("  %s\n", tui.StyleMuted.Render("skipped in dry-run mode"))
	}

	fmt.Println()

	// -------------------------------------------------------------------------
	// Section 5: Dotfiles
	// -------------------------------------------------------------------------
	fmt.Println(tui.SectionHeader("dotfiles", "wire from brain into ~/"))
	fmt.Println()

	dotfilesDir := filepath.Join(overseerDir, "dotfiles")
	if _, err := os.Stat(dotfilesDir); os.IsNotExist(err) {
		fmt.Printf("  %s\n", tui.StyleMuted.Render("no dotfiles found — add files under "+dotfilesDir+"/"))
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		if err := symlink.MakeAll(dotfilesDir, home, dryRun); err != nil {
			return fmt.Errorf("wiring dotfiles: %w", err)
		}
	}

	fmt.Println()

	// -------------------------------------------------------------------------
	// Section 5: Packages (macOS only)
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
	fmt.Println(tui.StyleMuted.Render("Next: edit "+filepath.Join(overseerDir, "config.yaml")+" to add integrations and git profiles."))

	return nil
}

// runBrainSetup wires dotfiles from brain and installs Brew packages.
// Shared by overseer brain setup.
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

	dotfilesDir := filepath.Join(brainOverseer, "dotfiles")
	if _, err := os.Stat(dotfilesDir); os.IsNotExist(err) {
		fmt.Println(tui.WarnLine("setup", "dotfiles not found in brain — run: overseer setup"))
	} else {
		if err := symlink.MakeAll(dotfilesDir, home, dry); err != nil {
			return fmt.Errorf("wiring dotfiles: %w", err)
		}
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

// runSetupBrainGit is called from the setup wizard to optionally initialize
// the brain as a git repository. It is a no-op if the brain is already a repo.
func runSetupBrainGit(cfg *config.Config, configuredURL string) error {
	brainPath := config.ResolveBrainPath(cfg)

	if brainIsGit(brainPath) {
		fmt.Printf("  %s  %s\n", tui.StyleOK.Render("✓"), tui.StyleNormal.Render("already a git repository — skipping"))
		return nil
	}

	confirmed, err := tui.Confirm("set up brain as a git repository?")
	if err != nil || !confirmed {
		fmt.Printf("  %s\n", tui.StyleMuted.Render("skipped — brain will remain a plain folder"))
		return nil
	}
	fmt.Println()

	idx, err := tui.Select("new brain or clone an existing repository?", []tui.SelectItem{
		{Title: "new brain", Subtitle: "git init + optional remote + initial commit"},
		{Title: "clone existing", Subtitle: "fetch from a remote and replace local files"},
	})
	if err != nil || idx == -1 {
		fmt.Printf("  %s\n", tui.StyleMuted.Render("skipped"))
		return nil
	}
	fmt.Println()

	switch idx {
	case 0:
		return initBrainRepo(cfg, configuredURL)
	case 1:
		return cloneExistingBrain(cfg, configuredURL)
	}
	return nil
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
