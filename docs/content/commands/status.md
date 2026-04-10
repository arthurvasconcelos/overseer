---
title: status
weight: 16
---

Health-check all configured integrations.

```bash
overseer status
overseer status --format json
```

Tests connectivity for each configured integration (1Password, Jira, Slack, Google, GitHub, GitLab) and reports whether it is reachable and authenticated.

## JSON output

```json
[
  { "name": "1password", "ok": true, "message": "signed in" },
  { "name": "jira/work", "ok": true, "message": "you@company.com" },
  { "name": "slack/work", "ok": true, "message": "workspace-name" },
  { "name": "github/personal", "ok": false, "message": "token expired" }
]
```

Useful for a quick sanity check after setting up a new machine or rotating credentials.
