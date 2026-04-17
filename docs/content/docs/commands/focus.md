---
title: focus
weight: 9
---

Start a timed focus session. Shows a live countdown in the terminal. Sends a desktop notification when the session ends. Optionally logs the elapsed time as a Jira worklog.

```bash
overseer focus              # 25-minute session (default)
overseer focus 45m          # 45-minute session
overseer focus 1h30m        # 90-minute session
overseer focus 90           # plain number = minutes
```

Duration formats accepted: `25m`, `1h`, `1h30m`, `90` (minutes).

## Jira time logging

Pass `--issue` to log the elapsed time to a Jira issue when the session ends. You will be prompted to confirm before the worklog is submitted.

```bash
overseer focus --issue PROJECT-123
overseer focus --issue PROJECT-123 --instance work   # pick a specific Jira instance
overseer focus 45m --issue PROJECT-123 --name "API design"
```

| Flag | Description |
|---|---|
| `--issue` | Jira issue key to log time against |
| `--name` | Label for the session (shown in the terminal header) |
| `--instance` | Jira instance name from config (defaults to first configured) |

If you stop the session early with `ctrl+c`, the elapsed time (minimum 1 minute) is used for the worklog prompt.
