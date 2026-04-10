---
title: TypeScript SDK
weight: 2
---

The TypeScript SDK provides helpers for reading the overseer context and applying consistent terminal styling.

Source: [`sdk/typescript/`](https://github.com/arthurvasconcelos/overseer/tree/main/sdk/typescript)

## Install

```bash
npm install @arthurvasconcelos/overseer-sdk
```

## Reading context

```typescript
import { loadContext, getSecret } from "@arthurvasconcelos/overseer-sdk";

const ctx = loadContext();

console.log(ctx.version);     // overseer version string
console.log(ctx.configPath);  // path to merged config file

// Retrieve a resolved secret declared in the sidecar manifest
const token = getSecret(ctx, "github.personal", "token");
```

`loadContext()` reads and parses the `OVERSEER_CONTEXT` environment variable injected by overseer at runtime.

## Styling

```typescript
import { styles, sectionHeader, warnLine } from "@arthurvasconcelos/overseer-sdk";

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

## Example plugin

```typescript
#!/usr/bin/env ts-node
import { loadContext, sectionHeader } from "@arthurvasconcelos/overseer-sdk";

const ctx = loadContext();
console.log(sectionHeader("My Plugin", ctx.version));
// ... your plugin logic
```

Compile to a single executable (e.g. with `esbuild` or `pkg`), name it `overseer-myplugin`, and drop it in `brain/overseer/plugins/`.
