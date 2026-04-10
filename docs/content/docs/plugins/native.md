---
title: Native plugins
weight: 3
---

Native plugins are compiled into the overseer binary. They differ from external plugins in that they:

- Require no external executables or published repos
- Can hook deeply into `daily` and `status` without process spawning
- Are enabled/disabled via config rather than by adding or removing executables

## Listing native plugins

```bash
overseer plugins
```

Shows all native plugins with their enabled/disabled state.

## Enabling and disabling

Each native plugin has a default enable condition (typically: enabled when its integration section is non-empty). Override this explicitly with `plugins.settings`:

```yaml
plugins:
  settings:
    jira:
      enabled: false   # disable even if integrations.jira[] is populated
    claude:
      enabled: true    # opt-in plugins require explicit enable
```

## Built-in plugins

### `jira`

Jira issue tracking integration.

- **Default**: enabled when `integrations.jira[]` is non-empty
- **daily**: shows open issues assigned to you, per instance
- **status**: auth ping per configured instance

### `slack`

Slack workspace integration.

- **Default**: enabled when `integrations.slack[]` is non-empty
- **daily**: shows recent mentions, per workspace
- **status**: auth ping per configured workspace

### `google`

Google Calendar integration.

- **Default**: enabled when `integrations.google[]` is non-empty
- **daily**: shows today's events, per account
- **status**: OAuth token validity per account

### `github`

GitHub pull request integration.

- **Default**: enabled when `integrations.github[]` is non-empty
- **Commands**: none (PRs are shown via `overseer prs`)
- **daily**: none
- **status**: none (visible in `overseer plugins` for awareness)

### `gitlab`

GitLab merge request integration.

- **Default**: enabled when `integrations.gitlab[]` is non-empty
- **Commands**: none (MRs are shown via `overseer prs`)
- **daily**: none
- **status**: none

### `obsidian`

Obsidian vault integration.

- **Default**: enabled when `obsidian.vault_path` is set
- **status**: checks that the vault path exists on disk

### `claude`

Claude AI configuration management. Manages symlinks between the brain and `~/.claude/`.

- **Default**: disabled — must be explicitly enabled
- **Commands**: `overseer claude setup`, `overseer claude list`
- **status**: symlink health for all managed Claude config targets

**Enable:**

```yaml
plugins:
  settings:
    claude:
      enabled: true
```

**Brain layout** (`<brain>/claude/`):

| Brain path | Local target | Link type |
|---|---|---|
| `claude/CLAUDE.md` | `~/.claude/CLAUDE.md` | file symlink |
| `claude/settings.json` | `~/.claude/settings.json` | file symlink |
| `claude/plans/` | `~/.claude/plans` | whole-dir symlink |
| `claude/memory/` | `~/.claude/memory` | whole-dir symlink |
| `claude/hooks/<name>` | `~/.claude/hooks/<name>` | per-file symlinks |
| `claude/skills/<name>/` | `~/.claude/skills/<name>` | per-dir symlinks |

Run `overseer claude setup` to adopt existing files, migrate old symlinks, and create any missing links. The wizard is safe to re-run — already correct symlinks are skipped.

## Writing a native plugin

Add a new package under `cli/internal/plugins/<name>/`:

```go
package myplugin

import (
    "github.com/arthurvasconcelos/overseer/internal/config"
    "github.com/arthurvasconcelos/overseer/internal/nativeplugin"
)

func init() {
    nativeplugin.Register(&nativeplugin.Plugin{
        Name:        "myplugin",
        Description: "What my plugin does",
        IsEnabled:   func(cfg *config.Config) bool { ... },
        DailyItems:  func(cfg *config.Config) []nativeplugin.DailyTask { ... },   // optional
        StatusChecks: func(cfg *config.Config) []nativeplugin.StatusCheckFn { ... }, // optional
        Commands:    func(cfg *config.Config) []*cobra.Command { ... },            // optional
    })
}
```

Then add a blank import to `cli/cmd/plugins_init.go`:

```go
_ "github.com/arthurvasconcelos/overseer/internal/plugins/myplugin"
```

That's it — the plugin is automatically wired into `daily`, `status`, and the help output.
