package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/arthurvasconcelos/overseer/internal/tui"
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

var contextCopy bool

func init() {
	contextCmd.Flags().BoolVar(&contextCopy, "copy", false, "Copy output to clipboard (macOS)")
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
	if output.Format == "json" {
		return runContextJSON()
	}
	if contextCopy {
		var buf bytes.Buffer
		writeContextMarkdown(&buf)
		fmt.Print(buf.String())
		if err := copyToClipboard(buf.String()); err != nil {
			fmt.Println(tui.WarnLine("copy", err.Error()))
		}
		return nil
	}
	writeContextMarkdown(os.Stdout)
	return nil
}

func runContextMarkdown() error {
	writeContextMarkdown(os.Stdout)
	return nil
}

func writeContextMarkdown(w io.Writer) {
	fmt.Fprintf(w, "# overseer %s\n\n", Version)
	fmt.Fprintln(w, strings.TrimSpace(rootCmd.Long))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Brain")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "The **brain** is a private git repo holding your personal config at `brain/overseer/config.yaml`.")
	fmt.Fprintln(w, "It is separate from the overseer binary so config can be version-controlled across machines.")
	fmt.Fprintln(w, "Brain path resolves via: `OVERSEER_BRAIN` env → `system.brain_path` in `config.local.yaml` → `brain.path` in brain config → `~/brain`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Two config files are merged (local overrides brain):")
	fmt.Fprintln(w, "1. `brain/overseer/config.yaml` — shared, portable, committed")
	fmt.Fprintln(w, "2. `~/.config/overseer/config.local.yaml` — machine-local overrides")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Secret fields accept `op://vault/item/field` references resolved at runtime via the 1Password CLI.")
	fmt.Fprintln(w, "Run `overseer accounts` to find your 1Password account IDs.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Global flags")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "- `--format text|json` — output format for commands that produce structured data (default: `text`)")
	fmt.Fprintln(w, "- `--version` — print version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Commands")
	fmt.Fprintln(w)

	for _, cmd := range rootCmd.Commands() {
		if !cmd.IsAvailableCommand() || cmd.Name() == "help" {
			continue
		}
		printCmdMarkdown(w, cmd, 3)
	}

	fmt.Fprintln(w, "## Config reference")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Full JSON Schema: `overseer config schema`")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Key | Description |")
	fmt.Fprintln(w, "|-----|-------------|")
	fmt.Fprintln(w, "| `secrets.environments` | Map of alias → 1Password account ID |")
	fmt.Fprintln(w, "| `integrations.jira[]` | Jira instances (name, base_url, email, token, op_account) |")
	fmt.Fprintln(w, "| `integrations.slack[]` | Slack workspaces (name, token, op_account) |")
	fmt.Fprintln(w, "| `integrations.google[]` | Google accounts (name, credentials_doc, op_account) |")
	fmt.Fprintln(w, "| `integrations.github[]` | GitHub accounts (name, token, op_account) |")
	fmt.Fprintln(w, "| `integrations.gitlab[]` | GitLab instances (name, base_url, token, op_account) |")
	fmt.Fprintln(w, "| `git.defaults` | Shared git settings (user_name, gpg_format, gpg_ssh_program, commit_gpgsign) |")
	fmt.Fprintln(w, "| `git.profiles[]` | Named git identities (name, email, signing_key, ...) |")
	fmt.Fprintln(w, "| `system.repos_path` | Where managed repos are cloned (machine-local) |")
	fmt.Fprintln(w, "| `system.brain_path` | Brain directory override (machine-local) |")
	fmt.Fprintln(w, "| `brain.path` | Canonical brain path (e.g. ~/brain) |")
	fmt.Fprintln(w, "| `brain.url` | Brain git remote URL |")
	fmt.Fprintln(w, "| `brain.git_profile` | Git profile to use for brain commits |")
	fmt.Fprintln(w, "| `obsidian.vault_path` | Path to the Obsidian vault |")
	fmt.Fprintln(w, "| `obsidian.vault_name` | Vault name as registered in Obsidian |")
	fmt.Fprintln(w, "| `obsidian.daily_notes_folder` | Folder for daily notes |")
	fmt.Fprintln(w, "| `obsidian.templates_folder` | Folder for note templates |")
	fmt.Fprintln(w, "| `brew.brewfile` | Brewfile path relative to repos_path |")
	fmt.Fprintln(w, "| `repos[]` | Managed repos (name, url, path, readonly, git_profile) |")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Plugin system")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Any binary named `overseer-<name>` on PATH or in `brain/plugins/` is auto-registered as `overseer <name>`.")
	fmt.Fprintln(w, "Plugin SDKs: `sdk/python/`, `sdk/typescript/`.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Environment variables")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Variable | Description |")
	fmt.Fprintln(w, "|----------|-------------|")
	fmt.Fprintln(w, "| `OVERSEER_BRAIN` | Override brain directory path |")
	fmt.Fprintln(w, "| `OVERSEER_REPOS_PATH` | Override repos root directory |")
}

func printCmdMarkdown(w io.Writer, cmd *cobra.Command, depth int) {
	heading := strings.Repeat("#", depth)
	fmt.Fprintf(w, "%s %s\n\n", heading, cmd.CommandPath())

	if cmd.Short != "" {
		fmt.Fprintln(w, cmd.Short)
		fmt.Fprintln(w)
	}
	if cmd.Long != "" {
		fmt.Fprintln(w, strings.TrimSpace(cmd.Long))
		fmt.Fprintln(w)
	}

	if cmd.HasAvailableLocalFlags() {
		fmt.Fprintln(w, "**Flags:**")
		fmt.Fprintln(w)
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			fmt.Fprintf(w, "- `--%s` (default: `%s`) — %s\n", f.Name, f.DefValue, f.Usage)
		})
		fmt.Fprintln(w)
	}

	if cmd.Example != "" {
		fmt.Fprintln(w, "**Examples:**")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, strings.TrimSpace(cmd.Example))
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	if cmd.HasAvailableSubCommands() {
		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() {
				printCmdMarkdown(w, sub, depth+1)
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

	return output.PrintJSON(manifest)
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
