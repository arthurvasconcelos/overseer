---
title: daily
weight: 7
---

Morning briefing: fetches and displays today's Jira tickets, Slack mentions, Google Calendar events — all in parallel.

```bash
overseer daily
```

Sections are contributed by **enabled native plugins**. Each plugin runs in its own goroutine and results are printed in a deterministic order. Integrations that are not enabled are silently skipped.

Built-in plugins that contribute to `daily`:

| Plugin | Config key | What it shows |
|---|---|---|
| `jira` | `integrations.jira[]` | Open issues assigned to you |
| `slack` | `integrations.slack[]` | Recent mentions |
| `google` | `integrations.google[]` | Today's calendar events |

External plugins can also contribute a section by declaring `"hooks": ["daily"]` in their [sidecar manifest](/docs/plugins/) and handling the `daily` argument when called.
