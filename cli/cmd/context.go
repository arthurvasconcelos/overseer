package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print a self-contained description of overseer for use in AI chats",
	Long: `Prints a comprehensive markdown document describing all overseer commands,
flags, config structure, and key concepts. Designed to be pasted directly
into an AI assistant chat as context.

Use --format json for a structured command manifest instead.`,
	Example: `  # Copy context to clipboard (macOS)
  overseer context | pbcopy

  # Save to a file
  overseer context > overseer-context.md

  # Structured manifest for programmatic use
  overseer context --format json`,
	RunE: runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

// commandJSON is the JSON representation of a single command for the manifest.
type commandJSON struct {
	Path        string        `json:"path"`
	Short       string        `json:"short"`
	Long        string        `json:"long,omitempty"`
	Example     string        `json:"example,omitempty"`
	Flags       []flagJSON    `json:"flags,omitempty"`
	Subcommands []commandJSON `json:"subcommands,omitempty"`
}

type flagJSON struct {
	Name    string `json:"name"`
	Default string `json:"default"`
	Usage   string `json:"usage"`
}

type contextManifest struct {
	Tool        string        `json:"tool"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Brain       string        `json:"brain"`
	Commands    []commandJSON `json:"commands"`
}

func runContext(_ *cobra.Command, _ []string) error {
	if outputFormat == "json" {
		return runContextJSON()
	}
	return runContextMarkdown()
}

func runContextMarkdown() error {
	fmt.Printf("# overseer %s\n\n", Version)
	fmt.Println(strings.TrimSpace(rootCmd.Long))
	fmt.Println()
	fmt.Println("## Brain")
	fmt.Println()
	fmt.Println("The **brain** is a private git repo holding your personal config at `brain/overseer/config.yaml`.")
	fmt.Println("It is separate from the overseer binary so config can be version-controlled across machines.")
	fmt.Println("Brain path resolves via: `OVERSEER_BRAIN` env → `system.brain_path` in `config.local.yaml` → `brain.path` in brain config → `~/brain`.")
	fmt.Println()
	fmt.Println("Two config files are merged (local overrides brain):")
	fmt.Println("1. `brain/overseer/config.yaml` — shared, portable, committed")
	fmt.Println("2. `~/.config/overseer/config.local.yaml` — machine-local overrides")
	fmt.Println()
	fmt.Println("Secret fields accept `op://vault/item/field` references resolved at runtime via the 1Password CLI.")
	fmt.Println("Run `overseer accounts` to find your 1Password account IDs.")
	fmt.Println()
	fmt.Println("## Global flags")
	fmt.Println()
	fmt.Println("- `--format text|json` — output format for commands that produce structured data (default: `text`)")
	fmt.Println("- `--version` — print version")
	fmt.Println()
	fmt.Println("## Commands")
	fmt.Println()

	for _, cmd := range rootCmd.Commands() {
		if !cmd.IsAvailableCommand() || cmd.Name() == "help" {
			continue
		}
		printCmdMarkdown(cmd, 3)
	}

	fmt.Println("## Config reference")
	fmt.Println()
	fmt.Println("Full JSON Schema: `overseer config schema`")
	fmt.Println()
	fmt.Println("| Key | Description |")
	fmt.Println("|-----|-------------|")
	fmt.Println("| `secrets.environments` | Map of alias → 1Password account ID |")
	fmt.Println("| `integrations.jira[]` | Jira instances (name, base_url, email, token, op_account) |")
	fmt.Println("| `integrations.slack[]` | Slack workspaces (name, token, op_account) |")
	fmt.Println("| `integrations.google[]` | Google accounts (name, credentials_doc, op_account) |")
	fmt.Println("| `integrations.github[]` | GitHub accounts (name, token, op_account) |")
	fmt.Println("| `integrations.gitlab[]` | GitLab instances (name, base_url, token, op_account) |")
	fmt.Println("| `git.defaults` | Shared git settings (user_name, gpg_format, gpg_ssh_program, commit_gpgsign) |")
	fmt.Println("| `git.profiles[]` | Named git identities (name, email, signing_key, ...) |")
	fmt.Println("| `system.repos_path` | Where managed repos are cloned (machine-local) |")
	fmt.Println("| `system.brain_path` | Brain directory override (machine-local) |")
	fmt.Println("| `brain.path` | Canonical brain path (e.g. ~/brain) |")
	fmt.Println("| `brain.url` | Brain git remote URL |")
	fmt.Println("| `brain.git_profile` | Git profile to use for brain commits |")
	fmt.Println("| `obsidian.vault_path` | Path to the Obsidian vault |")
	fmt.Println("| `obsidian.vault_name` | Vault name as registered in Obsidian |")
	fmt.Println("| `obsidian.daily_notes_folder` | Folder for daily notes |")
	fmt.Println("| `obsidian.templates_folder` | Folder for note templates |")
	fmt.Println("| `brew.brewfile` | Brewfile path relative to repos_path |")
	fmt.Println("| `repos[]` | Managed repos (name, url, path, readonly, git_profile) |")
	fmt.Println()
	fmt.Println("## Plugin system")
	fmt.Println()
	fmt.Println("Any binary named `overseer-<name>` on PATH or in `brain/plugins/` is auto-registered as `overseer <name>`.")
	fmt.Println("Plugin SDKs: `sdk/python/`, `sdk/typescript/`.")
	fmt.Println()
	fmt.Println("## Environment variables")
	fmt.Println()
	fmt.Println("| Variable | Description |")
	fmt.Println("|----------|-------------|")
	fmt.Println("| `OVERSEER_BRAIN` | Override brain directory path |")
	fmt.Println("| `OVERSEER_REPOS_PATH` | Override repos root directory |")

	return nil
}

func printCmdMarkdown(cmd *cobra.Command, depth int) {
	heading := strings.Repeat("#", depth)
	fmt.Printf("%s %s\n\n", heading, cmd.CommandPath())

	if cmd.Short != "" {
		fmt.Println(cmd.Short)
		fmt.Println()
	}
	if cmd.Long != "" {
		fmt.Println(strings.TrimSpace(cmd.Long))
		fmt.Println()
	}

	if cmd.HasAvailableLocalFlags() {
		fmt.Println("**Flags:**")
		fmt.Println()
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			fmt.Printf("- `--%s` (default: `%s`) — %s\n", f.Name, f.DefValue, f.Usage)
		})
		fmt.Println()
	}

	if cmd.Example != "" {
		fmt.Println("**Examples:**")
		fmt.Println()
		fmt.Println("```")
		fmt.Println(strings.TrimSpace(cmd.Example))
		fmt.Println("```")
		fmt.Println()
	}

	if cmd.HasAvailableSubCommands() {
		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() {
				printCmdMarkdown(sub, depth+1)
			}
		}
	}
}

func runContextJSON() error {
	manifest := contextManifest{
		Tool:        "overseer",
		Version:     Version,
		Description: rootCmd.Long,
		Brain:       "Private git repo at ~/brain (default) holding brain/overseer/config.yaml. Path resolves via OVERSEER_BRAIN env, system.brain_path, brain.path, then ~/brain.",
	}

	for _, cmd := range rootCmd.Commands() {
		if !cmd.IsAvailableCommand() || cmd.Name() == "help" {
			continue
		}
		manifest.Commands = append(manifest.Commands, buildCommandJSON(cmd))
	}

	return printJSON(manifest)
}

func buildCommandJSON(cmd *cobra.Command) commandJSON {
	c := commandJSON{
		Path:    cmd.CommandPath(),
		Short:   cmd.Short,
		Long:    strings.TrimSpace(cmd.Long),
		Example: strings.TrimSpace(cmd.Example),
	}

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		c.Flags = append(c.Flags, flagJSON{
			Name:    f.Name,
			Default: f.DefValue,
			Usage:   f.Usage,
		})
	})

	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			c.Subcommands = append(c.Subcommands, buildCommandJSON(sub))
		}
	}

	return c
}
