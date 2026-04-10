---
title: mcp
weight: 10
---

Start a local [Model Context Protocol](https://modelcontextprotocol.io) server over stdio, letting AI assistants connect to overseer's data and run commands.

```bash
overseer mcp
```

## Setup with Claude

Add overseer as an MCP server in `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "overseer": {
      "command": "overseer",
      "args": ["mcp"]
    }
  }
}
```

## Available MCP tools

| Tool | Description |
|---|---|
| `list_commands` | List all overseer commands with descriptions |
| `run_prs` | Fetch open PRs and MRs from configured GitHub/GitLab instances |
| `run_repos_status` | Show git status for all managed repos |
| `get_config` | Return the active config as JSON |
| `run_command` | Run a shell command with secrets injected |
| `run_note_search` | Search the Obsidian vault |

### `run_command` parameters

| Parameter | Required | Description |
|---|---|---|
| `command` | Yes | Shell command to run (executed via `sh -c`) |
| `gitlab` | No | GitLab instance name — injects `GITLAB_TOKEN` and `GITLAB_HOST` |
| `github` | No | GitHub instance name — injects `GITHUB_TOKEN` |
| `env` | No | 1Password environment name — injects its secrets as env vars |

## Alternative: context dump

For a one-off paste into an AI chat rather than persistent integration, use [`overseer context`](/docs/commands/context).
