---
title: TypeScript SDK
weight: 2
---

The TypeScript SDK provides helpers for reading the overseer context and applying consistent terminal styling.

[![npm](https://img.shields.io/npm/v/overseer-sdk)](https://www.npmjs.com/package/overseer-sdk)

Source: [`sdk/typescript/`](https://github.com/arthurvasconcelos/overseer/tree/main/sdk/typescript)

## Install

```bash
npm install overseer-sdk
```

## Reading context

```typescript
import { loadContext, getSecret } from "overseer-sdk";

const ctx = loadContext();

console.log(ctx.version);      // overseer version string
console.log(ctx.config_path);  // path to merged config file

// Retrieve a resolved secret declared in the sidecar manifest
const token = getSecret(ctx, "github.personal", "token");
```

`loadContext()` reads and parses the `OVERSEER_CONTEXT` environment variable injected by overseer at runtime.

## Styling

```typescript
import { sectionHeader, warnLine, okLine, errorLine } from "overseer-sdk";

// Section header
console.log(sectionHeader("Deployments", "production"));
// → ▸ Deployments  ·  production

// Warning line
console.log(warnLine("auth", "token expired"));
// → ⚠  auth: token expired
```

### Color tokens

| Token | Code | Color | Usage |
|---|---|---|---|
| `styles.header` | 99 | Purple | Section titles (bold) |
| `styles.accent` | 212 | Pink | Keys, channels, usernames |
| `styles.ok` | 82 | Green | Success states (bold) |
| `styles.warn` | 214 | Amber | Warnings (bold) |
| `styles.error` | 196 | Red | Errors (bold) |
| `styles.muted` | 240 | Dark grey | Hints, empty state |
| `styles.dim` | 245 | Grey | Secondary info |
| `styles.normal` | 252 | Light | Body text |

All color codes use the xterm 256-color palette, which works in any modern terminal.

## Hooks

Use `runMain` to wire up your plugin's `daily` and `status` hooks:

```typescript
import { runMain, getSecret } from "overseer-sdk";
import type { PluginContext, StatusResult } from "overseer-sdk";

async function daily(ctx: PluginContext): Promise<string> {
  const token = getSecret(ctx, "myservice", "token");
  return "▸ My Service\n  all good\n";
}

async function status(ctx: PluginContext): Promise<StatusResult[]> {
  return [{ name: "myservice", ok: true, message: "connected" }];
}

runMain({ daily, status });
```

## Example plugin

Compile your script to a single executable (e.g. with `esbuild`), name it `overseer-myplugin`, and drop it in `brain/overseer/plugins/`.

```typescript
import { loadContext, sectionHeader } from "overseer-sdk";

const ctx = loadContext();
console.log(sectionHeader("My Plugin", ctx.version));
// ... your plugin logic
```
