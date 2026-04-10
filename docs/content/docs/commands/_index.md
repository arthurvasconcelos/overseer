---
title: Commands
weight: 3
---

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
| [`accounts`](/docs/commands/accounts) | List 1Password accounts signed into the `op` CLI |
| [`brain`](/docs/commands/brain) | Manage the brain directory |
| [`brew`](/docs/commands/brew) | Manage Homebrew packages via Brewfile |
| [`completion`](/docs/commands/completion) | Generate shell completion scripts |
| [`config`](/docs/commands/config) | Show active config and JSON Schema |
| [`context`](/docs/commands/context) | Print a self-contained AI-friendly description of overseer |
| [`daily`](/docs/commands/daily) | Morning briefing: Jira, Slack, Calendar, PRs |
| [`git`](/docs/commands/git) | Git identity management |
| [`init`](/docs/commands/init) | Create `~/.config/overseer/config.local.yaml` interactively |
| [`mcp`](/docs/commands/mcp) | Start MCP server for AI assistant integration |
| [`note`](/docs/commands/note) | Obsidian vault integration |
| [`prs`](/docs/commands/prs) | Open PRs across GitHub and GitLab |
| [`repos`](/docs/commands/repos) | Manage and sync git repos |
| [`run`](/docs/commands/run) | Run a command with secrets injected |
| [`setup`](/docs/commands/setup) | Interactive bootstrap wizard |
| [`status`](/docs/commands/status) | Health-check all integrations |
| [`update`](/docs/commands/update) | Self-update the binary |
