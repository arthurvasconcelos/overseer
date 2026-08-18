# Plugins


overseer supports two types of plugins: **native plugins** compiled into the binary, and **external plugins** discovered as executables on PATH or in the brain.

## Native plugins

Native plugins ship with the overseer binary and can be enabled or disabled via config. They integrate deeply with built-in commands (`daily`, `status`) through declared extension points.

See [Native plugins](/plugins/native) for the full reference.

## External plugins

Any executable named `overseer-<name>` is automatically registered as `overseer <name>` with no configuration required.

overseer searches for plugins in two places (in order):

1. `brain/overseer/plugins/` — your private brain plugins, not requiring PATH changes
2. Anywhere on `PATH`

The first match wins. Plugin executables can be written in any language.

### Naming

```
overseer-deploy   →   overseer deploy
overseer-standup  →   overseer standup
```

### Context injection

Before running a plugin, overseer injects an `OVERSEER_CONTEXT` environment variable containing a JSON payload with:

- `version` — the running overseer version
- `config_path` — path to the merged config file
- `secrets` — a map of resolved secrets declared in the sidecar manifest

### Sidecar manifest (optional)

Place `overseer-<name>.json` alongside the binary to declare metadata:

```json
{
  "description": "Deploy to production",
  "secrets": ["github.personal", "gitlab.work"],
  "hooks": ["daily", "status"],
  "tools": [
    {
      "name": "history",
      "description": "List the last 20 production deploys with their status",
      "args": ["history", "--limit", "20"]
    }
  ]
}
```

| Field | Description |
|---|---|
| `description` | Shown in `overseer --help` and `overseer plugins` |
| `secrets` | Integration references whose tokens overseer resolves and injects via `OVERSEER_CONTEXT` |
| `hooks` | Extension points to participate in: `"daily"` and/or `"status"` |
| `tools` | MCP tools the plugin contributes to `overseer mcp` |

#### `daily` hook

When `hooks` includes `"daily"`, overseer calls `overseer-<name> daily` during `overseer daily`. The plugin's stdout is printed as a section in the briefing output.

#### `status` hook

When `hooks` includes `"status"`, overseer calls `overseer-<name> status` during `overseer status`. The plugin must output a JSON array:

```json
[{ "name": "my-check", "ok": true, "message": "all good" }]
```

Each item is displayed as a status row alongside built-in checks.

### MCP tools

Everything a plugin declares in `tools` is registered as a tool on the [`overseer mcp`](/commands/mcp) server, so an AI assistant can call it the same way it calls a built-in one.

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Lowercase letters, digits, and underscores. Exposed as `<plugin>_<name>` |
| `description` | No | What the assistant sees. Defaults to the command line being run |
| `args` | No | Argv passed to the plugin binary. Defaults to `[name]` |

Give `args` explicitly whenever the tool name is not the command:

```json
{ "name": "wt_list", "description": "List all worktrees", "args": ["wt", "list"] }
```

A declared tool takes no parameters. The manifest is static, so a call has to mean the same thing every time for its description to stay honest — anything that needs arguments is better left to the `run_command` tool, which runs an arbitrary shell command. That makes `tools` the right place for read-only commands worth calling unprompted: status summaries, listings, health checks.

The plugin is run exactly as it is from the terminal, with `OVERSEER_CONTEXT` injected, and whatever it prints to stdout becomes the tool result. Since stdout is a pipe rather than a terminal, colouring should switch itself off — most CLI libraries handle that on their own. On a non-zero exit, stderr is returned to the assistant as the error.

Check what a plugin currently contributes with `overseer plugins`:

```
▸ external plugins
  p24  Platform24 repo index — clone, sync, and manage P24 GitLab repos
       mcp tools: p24_repos, p24_unpushed, p24_check, p24_wt_list
```

Tools are read from the manifest when the MCP server starts, so an assistant already holding a session needs to reconnect before an edited manifest takes effect.

## Listing plugins

```bash
overseer plugins
```

Shows all native plugins and their enabled/disabled state, followed by any discovered external plugins.

## SDKs

Plugin SDKs are available for Python and TypeScript to simplify reading context and applying consistent styling.

- [Python SDK](/plugins/python) — `pip install overseer-sdk`
- [TypeScript SDK](/plugins/typescript) — `npm install overseer-sdk`
- [Native plugins](/plugins/native) — Built-in plugin reference
