---
description: Gather a day's calendar, commits, merge requests, Jira issues and Claude Code sessions into one structured payload.
---

# journal

Collect everything overseer knows about one day, so an assistant can turn it into
a daily note. This command **only gathers** — it writes nothing and produces no
prose. The note itself is written by the assistant, which can take corrections
and fold in meeting notes that no API knows about.

```bash
overseer journal context
overseer journal context --date 2026-08-18
overseer journal context --date 2026-08-18 --format json
```

## What it gathers

| Section | Source | Window |
|---|---|---|
| `calendar` | Google Calendar, all configured accounts | the whole day |
| `commits` | git, across managed repos | authored that day by a configured git identity |
| `mrs` | GitLab / GitHub | merged that day, plus MRs opened that day and still open |
| `jira` | Jira | issues assigned to you and updated that day |
| `worklog` | `overseer claude worklog` | Claude Code sessions captured that day |
| `learning` | learning database | entries added that day |

It also resolves `note_path` — where the day's note lives in the vault — and
`note_exists`, so the caller knows whether to create or merge.

## Degrading rather than failing

Every remote source runs concurrently under its own deadline, and a source that
fails contributes a line to `warnings` instead of failing the command. A stale
calendar token still leaves you with commits, MRs and sessions.

## Why merge requests are queried per project

The instance-wide merged-MR search (`scope=created_by_me`) is expensive enough
that gitlab.com answers it with `408 Request Timeout` for accounts with many
projects. `journal context` instead resolves the GitLab project of each repo
that actually saw activity that day — from commits and from session working
directories — and queries those projects directly. That keeps the fan-out to a
handful of fast requests. When no local repo maps to an instance, it falls back
to the instance-wide search.

## Feeding the daily note

Pair with `overseer claude worklog` for session capture. The intended flow is:

1. The `SessionEnd` hook captures each Claude Code session as it ends.
2. At the end of the day, `journal context` assembles the full picture.
3. An assistant merges that into the Obsidian daily note.
