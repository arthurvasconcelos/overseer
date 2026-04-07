package cmd

import (
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

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate config.local.yaml interactively",
	Long:  "Creates the machine-local config file (config.local.yaml) by prompting for system-specific settings.",
	RunE:  runInit,
}

func init() {
	initCmd.Hidden = true
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, _ []string) error {
	localPath, err := config.LocalPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(localPath); err == nil {
		fmt.Printf("config.local.yaml already exists at %s\n\n", fileLink(localPath))
		idx, err := tui.Select("overwrite it?", []tui.SelectItem{
			{Title: "yes", Subtitle: "replace the existing file"},
			{Title: "no", Subtitle: "keep the existing file and exit"},
		})
		if err != nil || idx != 0 {
			fmt.Println("aborted — no changes made")
			return nil
		}
		fmt.Println()
	}

	brainPath := defaultBrainPath()
	overseerHome := defaultOverseerHome()
	gpgSSHProgram := defaultGPGSSHProgram()

	fmt.Println("brain path — directory where your personal config, dotfiles, and Brewfile live")
	brainPath, err = tui.Prompt("brain_path", brainPath, brainPath)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Println("overseer home — directory where overseer clones managed repos")
	overseerHome, err = tui.Prompt("overseer_home", overseerHome, overseerHome)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Println("SSH signing program — used for git commit signing (leave blank to skip)")
	gpgSSHProgram, err = tui.Prompt("gpg_ssh_program", gpgSSHProgram, gpgSSHProgram)
	if err != nil {
		return err
	}
	fmt.Println()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	content := buildLocalConfig(brainPath, overseerHome, gpgSSHProgram)
	if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing config.local.yaml: %w", err)
	}

	fmt.Printf("wrote %s\n", fileLink(localPath))
	return nil
}

func buildLocalConfig(brainPath, overseerHome, gpgSSHProgram string) string {
	var sb strings.Builder
	sb.WriteString("system:\n")
	if brainPath != "" {
		sb.WriteString(fmt.Sprintf("    brain_path: %s\n", brainPath))
	}
	if overseerHome != "" {
		sb.WriteString(fmt.Sprintf("    overseer_home: %s\n", overseerHome))
	}
	if gpgSSHProgram != "" {
		sb.WriteString(fmt.Sprintf("    gpg_ssh_program: %s\n", gpgSSHProgram))
	}
	return sb.String()
}

// defaultBrainPath returns the default brain directory path.
func defaultBrainPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "brain"
	}
	return filepath.Join(home, "brain")
}

// defaultOverseerHome tries to detect a sensible default for overseer_home.
// It prefers the parent of the binary's directory, falling back to the cwd.
func defaultOverseerHome() string {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Dir(filepath.Dir(exe))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

// defaultGPGSSHProgram returns the platform-appropriate default SSH signing program.
func defaultGPGSSHProgram() string {
	switch runtime.GOOS {
	case "darwin":
		candidates := []string{
			"/Applications/1Password.app/Contents/MacOS/op-ssh-sign",
			"/opt/homebrew/bin/op-ssh-sign",
		}
		for _, c := range candidates {
			if _, err := exec.LookPath(c); err == nil {
				return c
			}
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	case "linux":
		if path, err := exec.LookPath("op-ssh-sign"); err == nil {
			return path
		}
	}
	return ""
}
