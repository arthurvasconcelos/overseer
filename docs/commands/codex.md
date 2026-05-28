---
description: Manage Codex configuration — sync brain/codex/ symlinks to ~/.codex/ and manage Codex skills.
---

# codex

Manages symlinks between `<brain>/codex/` and `~/.codex/`, keeping your Codex configuration version-controlled in the brain. Also provides a skill manager for discovering and activating Codex skills.

::: info Plugin — opt-in
Enable this plugin in your brain config:

```yaml
plugins:
  settings:
    codex:
      enabled: true
```
:::

## Managed targets

The plugin tracks these items and keeps them symlinked from the brain into `~/.codex/`:

| Target | Brain path | Link style |
|---|---|---|
| `AGENTS.md` | `brain/codex/AGENTS.md` | single symlink |
| `config.toml` | `brain/codex/config.toml` | single symlink |
| `plans/` | `brain/codex/plans/` | whole directory |
| `memories/` | `brain/codex/memories/` | whole directory |
| `hooks/` | `brain/codex/hooks/` | each child individually |
| `skills/` | `brain/codex/skills/` | each child individually |
| `scripts/` | `brain/codex/scripts/` | each child individually |

## `overseer codex setup`

Interactive wizard that adopts existing Codex config into the brain and creates the symlinks. Safe to re-run — already correct links are skipped.

```bash
overseer codex setup
overseer codex setup --dry-run   # preview without making changes
```

The wizard scans each managed target and shows what action it will take:

| Action | Meaning |
|---|---|
| `skip` | link is already correct |
| `link` | brain file exists, local missing — creates symlink |
| `adopt` | local file exists (not a symlink) — moves it to brain, then symlinks |
| `migrate` | symlink points to old brain path — moves brain file, updates symlink |
| `conflict` | both locations have content — manual resolution needed |

## `overseer codex list`

List the symlink status of all managed targets without making any changes.

```bash
overseer codex list
```

## `overseer codex skills`

Manage Codex skills. Skills are directories containing `.md` files with a YAML frontmatter `description` field, placed in `<brain>/codex/skills/` or an external search path. Enabling a skill creates a symlink in `~/.codex/skills/` so Codex can discover it.

### list

Show all available skills with their status.

```bash
overseer codex skills list
```

Status icons:
- `✓` active (symlink in place)
- `○` disabled
- `!` in brain but not yet synced — run `sync`
- `·` available from an external path, not yet enabled

### sync

Sync skill symlinks to match the current enabled/disabled state. Brain skills are linked automatically; external skills are only linked if you have previously enabled them.

```bash
overseer codex skills sync
```

### enable / disable

Enable or disable skills by name. Enabling creates the symlink in `~/.codex/skills/`; disabling removes it.

```bash
overseer codex skills enable my-skill
overseer codex skills enable skill-a skill-b   # multiple at once
overseer codex skills enable                   # interactive multi-select

overseer codex skills disable my-skill
overseer codex skills disable                  # interactive multi-select
```

### list-path

List all registered external skill search paths.

```bash
overseer codex skills list-path
```

### add-path

Register a directory as an additional skill search path. If the given directory contains a `skills/` subdirectory, that subdirectory is used automatically.

```bash
overseer codex skills add-path ~/repos/my-ai-config
overseer codex skills add-path ~/repos/my-ai-config/skills
```

After adding, the command prints the discovered skills available to enable.

### remove-path

Remove a directory from the skill search paths. Also removes any active symlinks in `~/.codex/skills/` that pointed into that directory.

```bash
overseer codex skills remove-path ~/repos/my-ai-config/skills
overseer codex skills remove-path   # interactive select
```

## `overseer codex mcp`

Manage the Codex MCP server registration for overseer. These commands
delegate to `codex mcp add/remove` and write to `~/.codex/config.toml`.

```bash
overseer codex mcp install    # register — idempotent
overseer codex mcp uninstall  # remove   — idempotent
```

Running `install` when already registered, or `uninstall` when not registered,
prints an informative message and exits cleanly.

The install command writes the full binary path so Codex can find it
regardless of the PATH environment it inherits.

## Config

External skill search paths are stored in the brain config under `integrations.codex.skill_search_paths`:

```yaml
integrations:
  codex:
    skill_search_paths:
      - ~/repos/my-ai-config/skills
```

Disabled skill names are persisted in `<brain>/codex/skills-state.json` (managed automatically — do not edit by hand).
