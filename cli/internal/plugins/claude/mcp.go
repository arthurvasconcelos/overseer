package claude

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

const mcpServerKey = "overseer"

func claudeMCPCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "mcp",
		Short: "Manage Claude Code MCP server registration",
	}
	root.AddCommand(claudeMCPInstallCmd())
	root.AddCommand(claudeMCPUninstallCmd())
	return root
}

func claudeMCPInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Register overseer as an MCP server in Claude Code (user scope)",
		RunE:  runClaudeMCPInstall,
	}
}

func claudeMCPUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove overseer from Claude Code MCP servers",
		RunE:  runClaudeMCPUninstall,
	}
}

func runClaudeMCPInstall(_ *cobra.Command, _ []string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	if exec.Command("claude", "mcp", "get", mcpServerKey).Run() == nil {
		fmt.Println("overseer is already registered as an MCP server")
		return nil
	}

	cmd := exec.Command("claude", "mcp", "add",
		"--transport", "stdio",
		"--scope", "user",
		mcpServerKey, "--", self, "mcp",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("registering MCP server (is the claude CLI installed and on PATH?): %w", err)
	}
	fmt.Println("Registered overseer as an MCP server (available in all projects)")
	return nil
}

func runClaudeMCPUninstall(_ *cobra.Command, _ []string) error {
	if exec.Command("claude", "mcp", "get", mcpServerKey).Run() != nil {
		fmt.Println("overseer is not registered as an MCP server")
		return nil
	}

	cmd := exec.Command("claude", "mcp", "remove", mcpServerKey)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing MCP server: %w", err)
	}
	fmt.Println("Removed overseer from MCP servers")
	return nil
}
