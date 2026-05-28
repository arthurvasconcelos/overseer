package codex

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

const mcpServerKey = "overseer"

func codexMCPCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "mcp",
		Short: "Manage Codex MCP server registration",
	}
	root.AddCommand(codexMCPInstallCmd())
	root.AddCommand(codexMCPUninstallCmd())
	return root
}

func codexMCPInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Register overseer as an MCP server in Codex",
		RunE:  runCodexMCPInstall,
	}
}

func codexMCPUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove overseer from Codex MCP servers",
		RunE:  runCodexMCPUninstall,
	}
}

func runCodexMCPInstall(_ *cobra.Command, _ []string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	if exec.Command("codex", "mcp", "get", mcpServerKey).Run() == nil {
		fmt.Println("overseer is already registered as an MCP server")
		return nil
	}

	cmd := exec.Command("codex", "mcp", "add", mcpServerKey, "--", self, "mcp")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("registering MCP server (is the codex CLI installed and on PATH?): %w", err)
	}
	fmt.Println("Registered overseer as a Codex MCP server")
	return nil
}

func runCodexMCPUninstall(_ *cobra.Command, _ []string) error {
	if exec.Command("codex", "mcp", "get", mcpServerKey).Run() != nil {
		fmt.Println("overseer is not registered as an MCP server")
		return nil
	}

	cmd := exec.Command("codex", "mcp", "remove", mcpServerKey)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing MCP server: %w", err)
	}
	fmt.Println("Removed overseer from MCP servers")
	return nil
}
