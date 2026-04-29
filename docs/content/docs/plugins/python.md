---
title: Python SDK
weight: 1
---

The Python SDK provides helpers for reading the overseer context and applying consistent terminal styling.

[![PyPI](https://img.shields.io/pypi/v/overseer-sdk)](https://pypi.org/project/overseer-sdk/)

Source: [`sdk/python/`](https://github.com/arthurvasconcelos/overseer/tree/main/sdk/python)

## Install

```bash
pip install overseer-sdk
```

## Reading context

```python
from overseer_sdk import PluginContext

ctx = PluginContext.from_env()

print(ctx.version)      # overseer version string
print(ctx.config_path)  # path to merged config file

# Retrieve a resolved secret declared in the sidecar manifest
token = ctx.secret("github.personal", "token")
```

`PluginContext.from_env()` reads and parses the `OVERSEER_CONTEXT` environment variable injected by overseer at runtime.

## Styling

```python
from overseer_sdk import section_header, warn_line, ok_line, error_line

# Section header
print(section_header("Deployments", "production"))
# → ▸ Deployments  ·  production

# Warning line
print(warn_line("auth", "token expired"))
# → ⚠  auth: token expired
```

### Color tokens

| Token | Code | Color | Usage |
|---|---|---|---|
| `STYLE_HEADER` | 99 | Purple | Section titles (bold) |
| `STYLE_ACCENT` | 212 | Pink | Keys, channels, usernames |
| `STYLE_OK` | 82 | Green | Success states (bold) |
| `STYLE_WARN` | 214 | Amber | Warnings (bold) |
| `STYLE_ERROR` | 196 | Red | Errors (bold) |
| `STYLE_MUTED` | 240 | Dark grey | Hints, empty state |
| `STYLE_DIM` | 245 | Grey | Secondary info |
| `STYLE_NORMAL` | 252 | Light | Body text |

All color codes use the xterm 256-color palette, which works in any modern terminal.

## Hooks

Use `run_main` to wire up your plugin's `daily` and `status` hooks:

```python
from overseer_sdk import PluginContext, StatusResult, run_main

def daily(ctx: PluginContext) -> str:
    token = ctx.secret("myservice", "token")
    return "▸ My Service\n  all good\n"

def status(ctx: PluginContext) -> list[StatusResult]:
    return [StatusResult(name="myservice", ok=True, message="connected")]

if __name__ == "__main__":
    run_main(daily_fn=daily, status_fn=status)
```

## Example plugin

Make the file executable and name it `overseer-myplugin`, then drop it in `brain/overseer/plugins/`.

```python
#!/usr/bin/env python3
from overseer_sdk import PluginContext, section_header

ctx = PluginContext.from_env()
print(section_header("My Plugin", ctx.version))
# ... your plugin logic
```
