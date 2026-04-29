# @overseer/sdk (TypeScript)

[![npm](https://img.shields.io/npm/v/@overseer/sdk)](https://www.npmjs.com/package/@overseer/sdk)
[![Node](https://img.shields.io/node/v/@overseer/sdk)](https://www.npmjs.com/package/@overseer/sdk)
[![Tests](https://img.shields.io/github/actions/workflow/status/arthurvasconcelos/overseer/ci.yml?branch=main&label=tests)](https://github.com/arthurvasconcelos/overseer/actions/workflows/ci.yml)

TypeScript SDK for building external plugins for [overseer](https://github.com/arthurvasconcelos/overseer) — a personal developer CLI that unifies daily workflows.

---

## What is an overseer plugin?

overseer supports external plugins: any executable named `overseer-<name>` on `PATH` or in `brain/overseer/plugins/` is automatically registered as `overseer <name>`. Plugins can also hook into the `daily` briefing and `status` health-check commands.

This SDK gives you the context, helpers, and styling primitives to write those plugins in TypeScript/JavaScript.

---

## Install

```bash
npm install @overseer/sdk
```

---

## Quick start

```typescript
#!/usr/bin/env node
import { loadContext, runMain, sectionHeader, okLine } from "@overseer/sdk";
import type { PluginContext, StatusResult } from "@overseer/sdk";

async function daily(ctx: PluginContext): Promise<string> {
  const token = getSecret(ctx, "myservice", "token");
  // ... fetch data using token ...
  const lines = [
    sectionHeader("My Service", "2 items"),
    okLine("all good"),
  ];
  return lines.join("\n");
}

async function status(ctx: PluginContext): Promise<StatusResult[]> {
  return [{ name: "myservice", ok: true, message: "connected" }];
}

runMain({ daily, status });
```

Save it as `overseer-myservice` (compiled or via `tsx`/`node --loader`), make it executable, and drop it in `brain/overseer/plugins/`. Add a sidecar manifest:

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

### `loadContext(): PluginContext`

Load the plugin context from the `OVERSEER_CONTEXT` environment variable that overseer injects before calling your plugin. Throws if the variable is not set.

```typescript
import { loadContext } from "@overseer/sdk";

const ctx = loadContext();

ctx.version      // string — overseer version
ctx.config_path  // string — path to the active config.yaml
ctx.secrets      // PluginSecrets — resolved secrets map
```

---

### `getSecret(ctx, ref, key): string`

Return a resolved secret value by integration ref and key. Throws if the ref or key is not found — this usually means the secret wasn't declared in the plugin manifest.

```typescript
import { getSecret } from "@overseer/sdk";

const token = getSecret(ctx, "github.personal", "token");
```

---

### `runMain(hooks: PluginHooks): Promise<void>`

Wire up your plugin's hooks from CLI arguments. Call this at the entry point of your script.

```typescript
import { runMain } from "@overseer/sdk";

runMain({ daily: myDaily, status: myStatus });
```

- When called with `daily`: runs `hooks.daily(ctx)`, writes the returned string to stdout.
- When called with `status`: runs `hooks.status(ctx)`, serialises the returned array to JSON on stdout.
- Either hook can be omitted if your plugin only implements one.

#### `PluginHooks`

```typescript
interface PluginHooks {
  daily?: (ctx: PluginContext) => string | Promise<string>;
  status?: (ctx: PluginContext) => StatusResult[] | Promise<StatusResult[]>;
}
```

---

### `StatusResult`

```typescript
interface StatusResult {
  name: string;
  ok: boolean;
  message: string;
}

// Example
const results: StatusResult[] = [
  { name: "github", ok: true, message: "authenticated as user@example.com" },
  { name: "jira", ok: false, message: "token expired" },
];
```

---

### `notify(title, message, subtitle?)`

Fire a native desktop notification via `overseer notify`.

```typescript
import { notify } from "@overseer/sdk";

notify("Deploy done", "my-service v1.2.3 is live");
notify("Build failed", "see CI logs", "my-repo");
```

---

### Styling

The SDK exposes the same colour palette and helpers used by the overseer CLI itself, built on [chalk](https://github.com/chalk/chalk).

```typescript
import {
  styles,
  sectionHeader, // "▸ Label  ·  badge"
  okLine,         // "✓  label: message"
  warnLine,       // "⚠  label: message"
  errorLine,      // "✗  label: message"
} from "@overseer/sdk";

console.log(sectionHeader("GitHub", "3 open PRs"));
console.log(okLine("auth", "user@example.com"));
console.log(warnLine("rate limit", "80% used"));
console.log(errorLine("token", "expired"));

// Raw style functions
console.log(styles.header("Section title"));
console.log(styles.accent("username"));
console.log(styles.muted("hint text"));
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
- [Python SDK](https://pypi.org/project/overseer-sdk/) — same SDK for Python plugins
