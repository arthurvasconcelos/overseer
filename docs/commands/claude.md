---
description: Manage Claude AI configuration — sync brain/claude/ symlinks to ~/.claude/ and manage Claude Code skills.
---

# claude

Manages symlinks between `<brain>/claude/` and `~/.claude/`, keeping your Claude Code configuration version-controlled in the brain. Also provides a skill manager for discovering and activating Claude Code skills.

::: info Plugin — opt-in
Enable this plugin in your brain config:

```yaml
plugins:
  settings:
    claude:
      enabled: true
```
:::

## Managed targets

The plugin tracks these items and keeps them symlinked from the brain into `~/.claude/`:

| Target | Brain path | Link style |
|---|---|---|
| `CLAUDE.md` | `brain/claude/CLAUDE.md` | single symlink |
| `settings.json` | `brain/claude/settings.json` | single symlink |
| `plans/` | `brain/claude/plans/` | whole directory |
| `memory/` | `brain/claude/memory/` | whole directory |
| `hooks/` | `brain/claude/hooks/` | each child individually |
| `skills/` | `brain/claude/skills/` | each child individually |
| `scripts/` | `brain/claude/scripts/` | each child individually |

## `overseer claude setup`

Interactive wizard that adopts existing Claude config into the brain and creates the symlinks. Safe to re-run — already correct links are skipped.

```bash
overseer claude setup
overseer claude setup --dry-run   # preview without making changes
```

The wizard scans each managed target and shows what action it will take:

| Action | Meaning |
|---|---|
| `skip` | link is already correct |
| `link` | brain file exists, local missing — creates symlink |
| `adopt` | local file exists (not a symlink) — moves it to brain, then symlinks |
| `migrate` | symlink points to old brain path — moves brain file, updates symlink |
| `conflict` | both locations have content — manual resolution needed |

## `overseer claude list`

List the symlink status of all managed targets without making any changes.

```bash
overseer claude list
```

## `overseer claude skills`

Manage Claude Code skills. Skills are directories containing `.md` files with a YAML frontmatter `description` field, placed in `<brain>/claude/skills/` or an external search path. Enabling a skill creates a symlink in `~/.claude/skills/` so Claude Code can discover it.

### list

Show all available skills with their status.

```bash
overseer claude skills list
```

Status icons:
- `✓` active (symlink in place)
- `○` disabled
- `!` in brain but not yet synced — run `sync`
- `·` available from an external path, not yet enabled

### sync

Sync skill symlinks to match the current enabled/disabled state. Brain skills are linked automatically; external skills are only linked if you have previously enabled them.

```bash
overseer claude skills sync
```

### enable / disable

Enable or disable skills by name. Enabling creates the symlink in `~/.claude/skills/`; disabling removes it.

```bash
overseer claude skills enable my-skill
overseer claude skills enable skill-a skill-b   # multiple at once
overseer claude skills enable                   # interactive multi-select

overseer claude skills disable my-skill
overseer claude skills disable                  # interactive multi-select
```

### list-path

List all registered external skill search paths.

```bash
overseer claude skills list-path
```

### add-path

Register a directory as an additional skill search path. If the given directory contains a `skills/` subdirectory, that subdirectory is used automatically.

```bash
overseer claude skills add-path ~/repos/my-ai-config
overseer claude skills add-path ~/repos/my-ai-config/skills
```

After adding, the command prints the discovered skills available to enable.

### remove-path

Remove a directory from the skill search paths. Also removes any active symlinks in `~/.claude/skills/` that pointed into that directory.

```bash
overseer claude skills remove-path ~/repos/my-ai-config/skills
overseer claude skills remove-path   # interactive select
```

## `overseer claude mcp`

Manage the Claude Code MCP server registration for overseer. These commands
delegate to `claude mcp add/remove` (user scope, available in all projects)
and write to `~/.claude.json`.

```bash
overseer claude mcp install    # register — idempotent
overseer claude mcp uninstall  # remove   — idempotent
```

Running `install` when already registered, or `uninstall` when not registered,
prints an informative message and exits cleanly.

The install command writes the full binary path so Claude Code can find it
regardless of the PATH environment it inherits.

## `overseer claude worklog`

Capture what happened in each Claude Code session, so the day can be reconstructed
later without re-reading transcripts. A single day routinely produces millions of
tokens of transcript across several sessions — far too much to summarise in one pass.

```bash
overseer claude worklog install     # register the SessionEnd hook
overseer claude worklog show        # today's captured sessions
overseer claude worklog show --date 2026-08-18 --format json
overseer claude worklog uninstall   # remove the hook
```

`install` writes `<brain>/claude/hooks/session-end-worklog.sh`, symlinks it into
`~/.claude/hooks/`, and registers a `SessionEnd` hook in `<brain>/claude/settings.json`.
Only the `hooks` key is rewritten — every other setting keeps its original order and
formatting. Both commands are idempotent, and `uninstall` restores the file exactly.

### What a record holds

One record per session **per date** — a session that resumes the next day produces two.
Records are written to `~/.local/share/overseer/worklog/YYYY-MM-DD.jsonl`, which is
machine-local and deliberately not brain-synced: the brain carries the hook, the machine
keeps the data.

| Field | Notes |
|---|---|
| `started_at` / `ended_at` | From record timestamps, never file mtime — sessions resume across days |
| `workdirs` | Every distinct `cwd` resolved to its git root and deduped; one session often spans several repos |
| `prompts` | Human prompts only — gated on `promptSource`, minus IDE notifications, slash-command echoes and hook output |
| `files` | From Claude Code's file history. **Partial by design**: edits made through Bash never appear, so treat git as authoritative |

Commits are deliberately *not* recorded here. Work frequently lands in repos that were
never a session `cwd`, so commits are gathered day-wide elsewhere rather than attributed
per session.

The hook never fails a session: a malformed payload or unreadable transcript prints to
stderr and exits 0.

## Config

External skill search paths are stored in the brain config under `integrations.claude.skill_search_paths`:

```yaml
integrations:
  claude:
    skill_search_paths:
      - ~/repos/my-ai-config/skills
```

Disabled skill names are persisted in `<brain>/claude/skills-state.json` (managed automatically — do not edit by hand).
