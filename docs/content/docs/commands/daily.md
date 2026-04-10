---
title: daily
weight: 7
---

Morning briefing: fetches and displays today's Jira tickets, Slack mentions, Google Calendar events, and open PRs — all in parallel.

```bash
overseer daily
```

Requires configured integrations in `config.yaml`:

- **Jira** — `integrations.jira[]` with `base_url`, `email`, `token`
- **Slack** — `integrations.slack[]` with `token`
- **Google Calendar** — `integrations.google[]` with `credentials_doc`
- **GitHub/GitLab** — `integrations.github[]` / `integrations.gitlab[]` for the PR section

Integrations that are not configured are silently skipped. You can configure as many or as few as you need.
