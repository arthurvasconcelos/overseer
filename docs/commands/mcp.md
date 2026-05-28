# mcp

Start a local [Model Context Protocol](https://modelcontextprotocol.io) server over stdio, letting AI assistants connect to overseer's data and run commands.

```bash
overseer mcp
```

## How it works

`overseer mcp` is **not meant to be run directly** in your terminal. It is a
stdio server: when launched, it blocks on stdin waiting for JSON-RPC messages
from an MCP client. If you run it manually you will see no output and your
prompt will not return — that is expected behaviour.

The intended flow:

1. Register overseer with your AI assistant once (see [Setup](#setup) below).
2. Open a new AI session. The assistant reads its config, spawns `overseer mcp`
   as a background subprocess, and pipes JSON-RPC messages through stdin/stdout.
3. Ask the AI anything that benefits from your personal data — it calls overseer
   tools silently and surfaces the results in the conversation.

## Setup

### Claude Code

Registration is handled by the `claude` plugin — see [`overseer claude mcp`](/commands/claude#overseer-claude-mcp).

```bash
overseer claude mcp install
overseer claude mcp uninstall
```

### Codex

Registration is handled by the `codex` plugin — see [`overseer codex mcp`](/commands/codex#overseer-codex-mcp).

```bash
overseer codex mcp install
overseer codex mcp uninstall
```

### Other AI assistants

Each assistant has its own config format. For any MCP-compatible client, point it at the `overseer mcp` stdio command:

```bash
# find the full binary path to use in the client's config
which overseer
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

For a one-off paste into an AI chat rather than persistent integration, use [`overseer context`](/commands/context).
