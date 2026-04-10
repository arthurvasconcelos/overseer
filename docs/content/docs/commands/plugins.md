---
title: plugins
weight: 17
---

List available native and external plugins.

```bash
overseer plugins
```

Shows all native plugins with their enabled/disabled state, then any external plugins discovered on PATH or in `brain/overseer/plugins/`.

```
▸ native plugins
  jira       ✓ enabled   Jira issue tracking
  slack      ✓ enabled   Slack mentions
  google     ✓ enabled   Google Calendar events
  obsidian   ✓ enabled   Obsidian vault
  github     ✓ enabled   GitHub pull requests
  gitlab     ✓ enabled   GitLab merge requests
  claude     ✗ disabled  Claude AI config management

▸ external plugins
  deploy     Deploy to production
```

See [Plugins](/docs/plugins/) for how to enable, disable, and author plugins.
