---
title: status
weight: 16
---

Health-check all configured integrations.

```bash
overseer status
overseer status --format json
```

Checks are contributed by **enabled native plugins** (and external plugins that declare `"hooks": ["status"]`). Each check runs in its own goroutine. Always includes a 1Password check regardless of plugin state.

Built-in plugins that contribute to `status`:

| Plugin | Checks |
|---|---|
| `jira` | Auth ping per configured instance |
| `slack` | Auth ping per configured workspace |
| `google` | OAuth token validity per account |
| `obsidian` | Vault path exists on disk |
| `claude` | Symlink health for managed Claude config files |

## JSON output

```json
[
  { "name": "1password", "ok": true, "message": "signed in (2 accounts)" },
  { "name": "jira/work", "ok": true, "message": "you@company.com" },
  { "name": "slack/work", "ok": true, "message": "@username" },
  { "name": "google/personal", "ok": false, "message": "token expired — run: overseer daily to refresh" },
  { "name": "claude", "ok": true, "message": "all links healthy (6 targets)" }
]
```

Useful for a quick sanity check after setting up a new machine or rotating credentials.
