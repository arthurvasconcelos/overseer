package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/config"
	"github.com/arthurvasconcelos/overseer/internal/tui"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "Manage SSH config profiles",
}

func init() {
	rootCmd.AddCommand(sshCmd)
	sshCmd.AddCommand(sshListCmd)
	sshCmd.AddCommand(sshShowCmd)
	sshCmd.AddCommand(sshUseCmd)
	sshCmd.AddCommand(sshSetupCmd)
}

var sshListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all SSH config profiles",
	Args:  cobra.NoArgs,
	RunE:  runSSHList,
}

var sshShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show the SSH config block for a profile",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSSHShow,
}

var sshUseCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Activate an SSH config profile",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSSHUse,
}

var sshSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Add the overseer Include directive to ~/.ssh/config",
	Args:  cobra.NoArgs,
	RunE:  runSSHSetup,
}

func runSSHList(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	profiles := cfg.SSH.Profiles
	if output.Format == "json" {
		type item struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}
		active := sshActiveName()
		out := make([]item, len(profiles))
		for i, p := range profiles {
			out[i] = item{Name: p.Name, Active: p.Name == active}
		}
		return output.PrintJSON(out)
	}
	if len(profiles) == 0 {
		fmt.Println(tui.StyleMuted.Render("no SSH profiles configured — add ssh.profiles to config.yaml"))
		return nil
	}
	active := sshActiveName()
	for _, p := range profiles {
		if p.Name == active {
			fmt.Printf("  %s %s\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(p.Name))
		} else {
			fmt.Printf("    %s\n", tui.StyleNormal.Render(p.Name))
		}
	}
	return nil
}

func runSSHShow(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	profile, err := pickSSHProfile(cfg, args)
	if err != nil {
		return err
	}
	if output.Format == "json" {
		type out struct {
			Name   string `json:"name"`
			Config string `json:"config"`
		}
		return output.PrintJSON(out{Name: profile.Name, Config: profile.Config})
	}
	fmt.Println(tui.SectionHeader("ssh: "+profile.Name, ""))
	fmt.Println()
	fmt.Print(profile.Config)
	if !strings.HasSuffix(profile.Config, "\n") {
		fmt.Println()
	}
	return nil
}

func runSSHUse(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	profile, err := pickSSHProfile(cfg, args)
	if err != nil {
		return err
	}
	activePath, err := config.SSHActivePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(activePath), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.WriteFile(activePath, []byte(profile.Config), 0o600); err != nil {
		return fmt.Errorf("writing active SSH config: %w", err)
	}
	if err := os.WriteFile(activePath+".name", []byte(profile.Name), 0o600); err != nil {
		return fmt.Errorf("writing active profile name: %w", err)
	}
	fmt.Printf("  %s ssh: active profile → %s\n", tui.StyleOK.Render("✓"), tui.StyleAccent.Render(profile.Name))
	fmt.Println("  " + tui.StyleMuted.Render(activePath))
	return nil
}

func runSSHSetup(_ *cobra.Command, _ []string) error {
	activePath, err := config.SSHActivePath()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	include := "Include " + activePath

	existing, _ := os.ReadFile(sshConfigPath)
	if strings.Contains(string(existing), include) {
		fmt.Println("  " + tui.StyleOK.Render("✓") + " Include directive already present in ~/.ssh/config")
		return nil
	}

	newContent := include + "\n"
	if len(existing) > 0 {
		newContent += "\n" + string(existing)
	}
	if err := os.MkdirAll(filepath.Dir(sshConfigPath), 0o700); err != nil {
		return fmt.Errorf("creating ~/.ssh: %w", err)
	}
	if err := os.WriteFile(sshConfigPath, []byte(newContent), 0o600); err != nil {
		return fmt.Errorf("writing ~/.ssh/config: %w", err)
	}
	fmt.Printf("  %s Added Include directive to ~/.ssh/config\n", tui.StyleOK.Render("✓"))
	fmt.Println("  " + tui.StyleMuted.Render("Run: overseer ssh use <name> to activate a profile"))
	return nil
}

func pickSSHProfile(cfg *config.Config, args []string) (*config.SSHProfile, error) {
	profiles := cfg.SSH.Profiles
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no SSH profiles configured")
	}
	if len(args) == 1 {
		for i := range profiles {
			if profiles[i].Name == args[0] {
				return &profiles[i], nil
			}
		}
		return nil, fmt.Errorf("SSH profile %q not found", args[0])
	}
	active := sshActiveName()
	items := make([]tui.SelectItem, len(profiles))
	for i, p := range profiles {
		subtitle := ""
		if p.Name == active {
			subtitle = tui.StyleOK.Render("active")
		}
		items[i] = tui.SelectItem{Title: p.Name, Subtitle: subtitle}
	}
	idx, err := tui.Select("Select SSH profile", items)
	if err != nil {
		return nil, err
	}
	if idx < 0 {
		return nil, fmt.Errorf("cancelled")
	}
	return &profiles[idx], nil
}

// sshActiveName returns the name of the currently active SSH profile, or "" if none.
func sshActiveName() string {
	activePath, err := config.SSHActivePath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(activePath + ".name")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
