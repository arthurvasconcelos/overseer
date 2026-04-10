package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI assistant integration",
	Long: `Starts a local MCP (Model Context Protocol) server over stdio.

AI assistants can connect to overseer's data and run commands.

Add to ~/.claude/settings.json:
  "mcpServers": {
    "overseer": {
      "command": "overseer",
      "args": ["mcp"]
    }
  }`,
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(_ *cobra.Command, _ []string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	s := server.NewMCPServer("overseer", Version)

	s.AddTool(
		mcp.NewTool("list_commands",
			mcp.WithDescription("List all available overseer commands with descriptions"),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcpListCommands(ctx)
		},
	)

	s.AddTool(
		mcp.NewTool("run_prs",
			mcp.WithDescription("Fetch open pull requests and merge requests from configured GitHub and GitLab instances"),
		),
		mcpSubcmd(self, "prs"),
	)

	s.AddTool(
		mcp.NewTool("run_repos_status",
			mcp.WithDescription("Show git status for all managed repositories"),
		),
		mcpSubcmd(self, "repos", "status"),
	)

	s.AddTool(
		mcp.NewTool("get_config",
			mcp.WithDescription("Return the active overseer configuration as JSON"),
		),
		mcpSubcmd(self, "config"),
	)

	s.AddTool(
		mcp.NewTool("run_command",
			mcp.WithDescription("Run a shell command with secrets injected from 1Password. Optionally specify a GitLab instance, GitHub instance, or 1Password environment to inject credentials as environment variables before the command runs."),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("Shell command to run (executed via sh -c)"),
			),
			mcp.WithString("gitlab",
				mcp.Description("GitLab instance name from config — injects GITLAB_TOKEN and GITLAB_HOST"),
			),
			mcp.WithString("github",
				mcp.Description("GitHub instance name from config — injects GITHUB_TOKEN"),
			),
			mcp.WithString("env",
				mcp.Description("1Password environment name from config (e.g. p24) — injects its secrets as env vars"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			toolArgs := req.GetArguments()
			command, _ := toolArgs["command"].(string)
			if command == "" {
				return mcp.NewToolResultError("command is required"), nil
			}
			overseerArgs := []string{"run"}
			if v, _ := toolArgs["gitlab"].(string); v != "" {
				overseerArgs = append(overseerArgs, "--gitlab", v)
			}
			if v, _ := toolArgs["github"].(string); v != "" {
				overseerArgs = append(overseerArgs, "--github", v)
			}
			if v, _ := toolArgs["env"].(string); v != "" {
				overseerArgs = append(overseerArgs, "--env", v)
			}
			overseerArgs = append(overseerArgs, "--", "sh", "-c", command)
			out, err := exec.CommandContext(ctx, self, overseerArgs...).CombinedOutput()
			output := strings.TrimSpace(string(out))
			if err != nil {
				if output != "" {
					return mcp.NewToolResultError(output), nil
				}
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(output), nil
		},
	)

	s.AddTool(
		mcp.NewTool("run_note_search",
			mcp.WithDescription("Search the Obsidian vault for notes matching a query"),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query string"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, _ := req.GetArguments()["query"].(string)
			if query == "" {
				return mcp.NewToolResultError("query is required"), nil
			}
			return mcpExec(ctx, self, "note", "search", "--format", "json", query)
		},
	)

	return server.ServeStdio(s)
}

// mcpSubcmd returns a tool handler that runs an overseer subcommand with --format json.
func mcpSubcmd(self string, args ...string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cmdArgs := make([]string, 0, len(args)+2)
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, "--format", "json")
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcpExec(ctx, self, cmdArgs...)
	}
}

// mcpExec runs the overseer binary with given args and returns stdout as a tool result.
func mcpExec(ctx context.Context, self string, args ...string) (*mcp.CallToolResult, error) {
	out, err := exec.CommandContext(ctx, self, args...).Output()
	if err != nil {
		var msg string
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			msg = strings.TrimSpace(string(exitErr.Stderr))
		} else {
			msg = err.Error()
		}
		return mcp.NewToolResultError(msg), nil
	}
	return mcp.NewToolResultText(strings.TrimSpace(string(out))), nil
}

type mcpCommandEntry struct {
	Name  string `json:"name"`
	Short string `json:"short"`
}

// mcpListCommands introspects cobra to list all available top-level commands.
func mcpListCommands(_ context.Context) (*mcp.CallToolResult, error) {
	var cmds []mcpCommandEntry
	for _, c := range rootCmd.Commands() {
		if !c.IsAvailableCommand() {
			continue
		}
		cmds = append(cmds, mcpCommandEntry{Name: c.Name(), Short: c.Short})
	}
	if cmds == nil {
		cmds = []mcpCommandEntry{}
	}
	b, err := json.MarshalIndent(cmds, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
