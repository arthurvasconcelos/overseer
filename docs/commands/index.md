---
description: Complete reference for all overseer commands — global flags, output formats, and an index of every available command.
---

# Commands


Full reference for all overseer subcommands.

## Global flags

| Flag | Default | Description |
|---|---|---|
| `--format` | `text` | Output format: `text` or `json` |
| `--version` | — | Print overseer version and exit |
| `--help` | — | Print help for any command |

All commands that produce structured data support `--format json`. JSON output always emits an empty array `[]` rather than `null` when there are no items.

## Command index

| Command | Description |
|---|---|
| [`accounts`](/commands/accounts) | List 1Password accounts signed into the `op` CLI |
| [`brain`](/commands/brain) | Manage the brain directory |
| [`brew`](/commands/brew) | Manage Homebrew packages via Brewfile |
| [`claude`](/commands/claude) | Manage Claude AI config symlinks and skills (plugin) |
| [`codex`](/commands/codex) | Manage Codex config symlinks and skills (plugin) |
| [`completion`](/commands/completion) | Generate shell completion scripts |
| [`config`](/commands/config) | Show active config and JSON Schema |
| [`context`](/commands/context) | Print a self-contained AI-friendly description of overseer |
| [`daily`](/commands/daily) | Morning briefing: Jira, Slack, Calendar, PRs |
| [`env`](/commands/env) | Manage environment variable profiles |
| [`focus`](/commands/focus) | Timed focus session with optional Jira time logging |
| [`git`](/commands/git) | Git identity management |
| [`init`](/commands/init) | Create `~/.config/overseer/config.local.yaml` interactively |
| [`mcp`](/commands/mcp) | Start MCP server for AI assistant integration |
| [`note`](/commands/note) | Obsidian vault integration |
| [`notify`](/commands/notify) | Fire a native OS desktop notification |
| [`plugins`](/commands/plugins) | List and toggle native plugins |
| [`prs`](/commands/prs) | Open PRs across GitHub and GitLab |
| [`repos`](/commands/repos) | Manage and sync git repos |
| [`run`](/commands/run) | Run a command with secrets injected |
| [`setup`](/commands/setup) | Interactive bootstrap wizard |
| [`ssh`](/commands/ssh) | Manage SSH config profiles |
| [`standup`](/commands/standup) | Synthesize yesterday's activity into a standup message |
| [`status`](/commands/status) | Health-check all integrations |
| [`update`](/commands/update) | Self-update the binary |
| [`weekly`](/commands/weekly) | Activity summary for the past 7 days |
