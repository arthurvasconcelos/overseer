---
title: standup
weight: 17
---

Synthesize yesterday's activity into a standup message. Pulls Jira issues updated yesterday, GitLab MRs, and GitHub PRs from all configured instances, then uses Claude to generate a concise standup summary.

```bash
overseer standup
overseer standup --post   # post the summary to Slack after generating
```

The generated text is printed to the terminal. With `--post` it is also sent to the Slack channel configured under `standup.channel` in `config.yaml`.

## What it aggregates

- **Jira** — issues where you are the assignee, updated in the last 24 hours
- **GitHub** — PRs you opened or reviewed yesterday
- **GitLab** — MRs you authored or reviewed yesterday

Results are passed to Claude (via the configured Claude API key) to produce a short, human-readable standup. If no Claude key is configured the raw list is printed instead.
