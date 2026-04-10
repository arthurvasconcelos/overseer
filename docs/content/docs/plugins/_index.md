---
title: Plugins
weight: 4
---

overseer supports two types of plugins: **native plugins** compiled into the binary, and **external plugins** discovered as executables on PATH or in the brain.

## Native plugins

Native plugins ship with the overseer binary and can be enabled or disabled via config. They integrate deeply with built-in commands (`daily`, `status`) through declared extension points.

See [Native plugins](/docs/plugins/native) for the full reference.

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
  "hooks": ["daily", "status"]
}
```

| Field | Description |
|---|---|
| `description` | Shown in `overseer --help` and `overseer plugins` |
| `secrets` | Integration references whose tokens overseer resolves and injects via `OVERSEER_CONTEXT` |
| `hooks` | Extension points to participate in: `"daily"` and/or `"status"` |

#### `daily` hook

When `hooks` includes `"daily"`, overseer calls `overseer-<name> daily` during `overseer daily`. The plugin's stdout is printed as a section in the briefing output.

#### `status` hook

When `hooks` includes `"status"`, overseer calls `overseer-<name> status` during `overseer status`. The plugin must output a JSON array:

```json
[{ "name": "my-check", "ok": true, "message": "all good" }]
```

Each item is displayed as a status row alongside built-in checks.

## Listing plugins

```bash
overseer plugins
```

Shows all native plugins and their enabled/disabled state, followed by any discovered external plugins.

## SDKs

Plugin SDKs are available for Python and TypeScript to simplify reading context and applying consistent styling.

{{< cards >}}
  {{< card link="/docs/plugins/python" title="Python SDK" subtitle="pip install overseer-sdk" >}}
  {{< card link="/docs/plugins/typescript" title="TypeScript SDK" subtitle="npm install @arthurvasconcelos/overseer-sdk" >}}
  {{< card link="/docs/plugins/native" title="Native plugins" subtitle="Built-in plugin reference" >}}
{{< /cards >}}
