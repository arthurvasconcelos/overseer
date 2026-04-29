# overseer-sdk (Python)

[![PyPI](https://img.shields.io/pypi/v/overseer-sdk)](https://pypi.org/project/overseer-sdk/)
[![Python](https://img.shields.io/pypi/pyversions/overseer-sdk)](https://pypi.org/project/overseer-sdk/)
[![Tests](https://img.shields.io/github/actions/workflow/status/arthurvasconcelos/overseer/ci.yml?branch=main&label=tests)](https://github.com/arthurvasconcelos/overseer/actions/workflows/ci.yml)

Python SDK for building external plugins for [overseer](https://github.com/arthurvasconcelos/overseer) — a personal developer CLI that unifies daily workflows.

---

## What is an overseer plugin?

overseer supports external plugins: any executable named `overseer-<name>` on `PATH` or in `brain/overseer/plugins/` is automatically registered as `overseer <name>`. Plugins can also hook into the `daily` briefing and `status` health-check commands.

This SDK gives you the context, helpers, and styling primitives to write those plugins in Python.

---

## Install

```bash
pip install overseer-sdk
```

Or with [uv](https://docs.astral.sh/uv/):

```bash
uv add overseer-sdk
```

---

## Quick start

```python
#!/usr/bin/env python3
from overseer_sdk import PluginContext, StatusResult, run_main, section_header, ok_line

def daily(ctx: PluginContext) -> str:
    token = ctx.secret("myservice", "token")
    # ... fetch data using token ...
    lines = [str(section_header("My Service", "2 items"))]
    lines.append(str(ok_line("all good")))
    return "\n".join(lines)

def status(ctx: PluginContext) -> list[StatusResult]:
    return [StatusResult(name="myservice", ok=True, message="connected")]

if __name__ == "__main__":
    run_main(daily_fn=daily, status_fn=status)
```

Save it as `overseer-myservice`, make it executable, and drop it in `brain/overseer/plugins/`. Add a sidecar manifest:

```json
// overseer-myservice.json
{
  "description": "My service integration",
  "secrets": ["myservice"],
  "hooks": ["daily", "status"]
}
```

---

## API reference

### `PluginContext`

Loaded from the `OVERSEER_CONTEXT` environment variable that overseer injects before calling your plugin.

```python
from overseer_sdk import PluginContext

ctx = PluginContext.from_env()

ctx.version      # str — overseer version
ctx.config_path  # str — path to the active config.yaml
ctx.secrets      # dict[str, dict[str, str]] — resolved secrets
```

#### `ctx.secret(ref, key) -> str`

Return a resolved secret value by integration ref and key. Raises `KeyError` if the ref or key is not found — this usually means the secret wasn't declared in the plugin manifest.

```python
token = ctx.secret("github.personal", "token")
```

---

### `run_main(daily_fn, status_fn)`

Wire up your plugin's hooks from CLI arguments. Call this at the bottom of your script.

```python
from overseer_sdk import run_main

run_main(daily_fn=my_daily, status_fn=my_status)
```

- When called with `daily`: runs `daily_fn(ctx)`, prints the returned string to stdout.
- When called with `status`: runs `status_fn(ctx)`, serialises the returned list to JSON on stdout.
- Either argument can be omitted if your plugin only implements one hook.

---

### `StatusResult`

```python
from overseer_sdk import StatusResult

StatusResult(name="github", ok=True, message="authenticated as user@example.com")
StatusResult(name="jira", ok=False, message="token expired")
```

---

### `notify(title, message, subtitle="")`

Fire a native desktop notification via `overseer notify`.

```python
from overseer_sdk import notify

notify("Deploy done", "my-service v1.2.3 is live")
notify("Build failed", "see CI logs", subtitle="my-repo")
```

---

### Styling

The SDK exposes the same colour palette and helpers used by the overseer CLI itself, built on [rich](https://github.com/Textualize/rich).

```python
from rich.console import Console
from overseer_sdk import (
    section_header,  # ▸ Label  ·  badge
    ok_line,         # ✓  label: message
    warn_line,       # ⚠  label: message
    error_line,      # ✗  label: message
    STYLE_HEADER, STYLE_ACCENT, STYLE_OK, STYLE_WARN,
    STYLE_ERROR, STYLE_MUTED, STYLE_DIM, STYLE_NORMAL,
)

console = Console()
console.print(section_header("GitHub", "3 open PRs"))
console.print(ok_line("auth", "user@example.com"))
console.print(warn_line("rate limit", "80% used"))
console.print(error_line("token", "expired"))
```

---

## How overseer calls your plugin

overseer sets the `OVERSEER_CONTEXT` environment variable to a JSON object before invoking your plugin binary:

```json
{
  "version": "1.2.3",
  "config_path": "/Users/you/brain/overseer/config.yaml",
  "secrets": {
    "myservice": {
      "token": "resolved-secret-value"
    }
  }
}
```

Secrets listed in the manifest are pre-resolved (including `op://` 1Password references) before the plugin is called. Your plugin never needs to touch the `op` CLI directly.

---

## Links

- [overseer](https://github.com/arthurvasconcelos/overseer) — the main CLI
- [TypeScript SDK](https://www.npmjs.com/package/overseer-sdk) — same SDK for TypeScript plugins
