---
title: Plugins
weight: 4
---

overseer can be extended with plugins: any executable named `overseer-<name>` is automatically registered as `overseer <name>` with no configuration required.

## How it works

overseer searches for plugins in two places (in order):

1. `brain/overseer/plugins/` — your private brain plugins, not requiring PATH changes
2. Anywhere on `PATH`

The first match wins. Plugin executables can be written in any language.

## Naming

```
overseer-deploy   →   overseer deploy
overseer-standup  →   overseer standup
```

## Context injection

Before running a plugin, overseer injects an `OVERSEER_CONTEXT` environment variable containing a JSON payload with:

- `version` — the running overseer version
- `config_path` — path to the merged config file
- `secrets` — a map of resolved secrets declared in the sidecar manifest

## Sidecar manifest (optional)

Place `overseer-<name>.json` alongside the binary to declare metadata:

```json
{
  "description": "Deploy to production",
  "secrets": ["github.personal", "gitlab.work"]
}
```

| Field | Description |
|---|---|
| `description` | Shown in `overseer --help` |
| `secrets` | Integration references whose tokens overseer resolves and injects via `OVERSEER_CONTEXT` |

## SDKs

Plugin SDKs are available for Python and TypeScript to simplify reading context and applying consistent styling.

{{< cards >}}
  {{< card link="/plugins/python" title="Python SDK" subtitle="pip install overseer-sdk" >}}
  {{< card link="/plugins/typescript" title="TypeScript SDK" subtitle="npm install @arthurvasconcelos/overseer-sdk" >}}
{{< /cards >}}
