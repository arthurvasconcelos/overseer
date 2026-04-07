package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)


var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Manage Homebrew packages via Brewfile",
}

var brewCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Show which Brewfile packages are missing",
	RunE:  runBrewCheck,
}

var brewInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install missing Brewfile packages",
	RunE:  runBrewInstall,
}

var brewDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Overwrite Brewfile with currently installed packages",
	RunE:  runBrewDump,
}

func init() {
	brewCmd.AddCommand(brewCheckCmd)
	brewCmd.AddCommand(brewInstallCmd)
	brewCmd.AddCommand(brewDumpCmd)
	rootCmd.AddCommand(brewCmd)
}

func brewfilePath(cfg *config.Config) string {
	if cfg.Brew.Brewfile != "" {
		// Explicit override in config — resolve relative to overseer_home.
		return repoRoot(resolveOverseerHome(cfg), cfg.Brew.Brewfile)
	}
	return fmt.Sprintf("%s/Brewfile", config.BrainOverseerPath(cfg))
}

// requireBrewfile checks that the Brewfile exists at the resolved path and
// prints a helpful error if it does not.
func requireBrewfile(cfg *config.Config) (string, bool) {
	path := brewfilePath(cfg)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println(tui.WarnLine("brew", fmt.Sprintf("Brewfile not found at %s — configure brain_path and run: overseer brain init", path)))
		return path, false
	}
	return path, true
}

// brewfilePaths returns the active Brewfile paths: always the main one,
// plus Brewfile.local if it exists alongside it.
// Returns nil if the main Brewfile does not exist.
func brewfilePaths(cfg *config.Config) []string {
	main, ok := requireBrewfile(cfg)
	if !ok {
		return nil
	}
	paths := []string{main}
	local := main + ".local"
	if _, err := os.Stat(local); err == nil {
		paths = append(paths, local)
	}
	return paths
}

func brewAvailable() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

func requireBrew() bool {
	if !brewAvailable() {
		if runtime.GOOS == "linux" {
			fmt.Println(tui.StyleMuted.Render("Homebrew is not available — Linux package management is not yet supported"))
		} else {
			fmt.Println(tui.StyleWarn.Render("⚠  Homebrew not found — install it from https://brew.sh"))
		}
		return false
	}
	return true
}

// --- check ---

func runBrewCheck(_ *cobra.Command, _ []string) error {
	if !requireBrew() {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	paths := brewfilePaths(cfg)
	if paths == nil {
		return nil
	}

	label := strings.Join(paths, " + ")
	fmt.Println(tui.SectionHeader("brew check", label))
	fmt.Println()

	totalMissing := 0
	allSatisfied := true

	for _, path := range paths {
		cmd := exec.Command("brew", "bundle", "check", "--verbose", "--file="+path)
		cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1")
		out, err := cmd.CombinedOutput()

		if err != nil {
			allSatisfied = false
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "→") {
					continue
				}
				clean := line
				for _, prefix := range []string{"→ Formula ", "→ Cask ", "→ Tap ", "→ "} {
					clean = strings.TrimPrefix(clean, prefix)
				}
				clean = strings.TrimSuffix(clean, " needs to be installed or updated.")
				fmt.Println("  " + tui.StyleError.Render("✗") + "  " + tui.StyleNormal.Render(clean))
				totalMissing++
			}
		}
	}

	fmt.Println()
	if allSatisfied {
		fmt.Println("  " + tui.StyleOK.Render("✓") + "  " + tui.StyleNormal.Render("all packages satisfied"))
	} else {
		fmt.Println("  " + tui.StyleError.Render(fmt.Sprintf("%d missing", totalMissing)) +
			"  " + tui.StyleMuted.Render("run: overseer brew install"))
	}
	return nil
}

// --- install ---

func runBrewInstall(_ *cobra.Command, _ []string) error {
	if !requireBrew() {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	paths := brewfilePaths(cfg)
	if paths == nil {
		return nil
	}

	label := strings.Join(paths, " + ")
	fmt.Println(tui.SectionHeader("brew install", label))
	fmt.Println()

	for _, path := range paths {
		cmd := exec.Command("brew", "bundle", "install", "--file="+path)
		cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("brew bundle install %s: %w", path, err)
		}
	}

	fmt.Println()
	fmt.Println(tui.StyleOK.Render("✓") + "  all packages installed")
	return nil
}

// --- dump ---

func runBrewDump(_ *cobra.Command, _ []string) error {
	if !requireBrew() {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// dump always targets the main Brewfile only — Brewfile.local is managed manually
	path, ok := requireBrewfile(cfg)
	if !ok {
		return nil
	}

	fmt.Printf("this will overwrite %s with all currently installed packages\n\n", tui.StyleAccent.Render(path))

	idx, err := tui.Select("continue?", []tui.SelectItem{
		{Title: "yes", Subtitle: "overwrite Brewfile"},
		{Title: "no", Subtitle: "cancel"},
	})
	if err != nil || idx != 0 {
		fmt.Println(tui.StyleMuted.Render("cancelled"))
		return nil
	}
	fmt.Println()

	dumpCmd := exec.Command("brew", "bundle", "dump", "--force", "--file="+path)
	dumpCmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1")
	out, err := dumpCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew bundle dump: %s", strings.TrimSpace(string(out)))
	}

	count := countBrewfileEntries(path)
	fmt.Printf("%s  wrote %s\n",
		tui.StyleOK.Render("✓"),
		tui.StyleNormal.Render(fmt.Sprintf("%s (%d entries)", path, count)),
	)
	return nil
}

func countBrewfileEntries(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}
