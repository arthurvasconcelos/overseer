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
| [`accounts`](/commands/accounts) | List 1Password accounts signed into the `op` CLI |
| [`brain`](/commands/brain) | Manage the brain directory |
| [`brew`](/commands/brew) | Manage Homebrew packages via Brewfile |
| [`completion`](/commands/completion) | Generate shell completion scripts |
| [`config`](/commands/config) | Show active config and JSON Schema |
| [`context`](/commands/context) | Print a self-contained AI-friendly description of overseer |
| [`daily`](/commands/daily) | Morning briefing: Jira, Slack, Calendar, PRs |
| [`git`](/commands/git) | Git identity management |
| [`init`](/commands/init) | Create `~/.config/overseer/config.local.yaml` interactively |
| [`mcp`](/commands/mcp) | Start MCP server for AI assistant integration |
| [`note`](/commands/note) | Obsidian vault integration |
| [`prs`](/commands/prs) | Open PRs across GitHub and GitLab |
| [`repos`](/commands/repos) | Manage and sync git repos |
| [`run`](/commands/run) | Run a command with secrets injected |
| [`setup`](/commands/setup) | Interactive bootstrap wizard |
| [`status`](/commands/status) | Health-check all integrations |
| [`update`](/commands/update) | Self-update the binary |
