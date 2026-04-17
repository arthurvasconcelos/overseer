---
title: weekly
weight: 19
---

Activity summary for the past 7 days. Aggregates Jira issues, GitHub PRs, and GitLab MRs from all configured instances and prints them grouped by source.

```bash
overseer weekly
overseer weekly --ai      # generate an AI-written narrative using Claude
```

## What it aggregates

- **Jira** — issues assigned to you, updated in the last 7 days
- **GitHub** — PRs you opened or reviewed in the last 7 days
- **GitLab** — MRs you authored or reviewed in the last 7 days

All sources run in parallel. Missing or unconfigured integrations are skipped silently.

## AI summary

With `--ai`, the aggregated data is sent to Claude to produce a short narrative suitable for a weekly status update or engineering report. Requires a Claude API key configured under `claude.api_key` in `config.yaml`.
