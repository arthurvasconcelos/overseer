---
title: note
weight: 11
---

Obsidian vault integration. Requires `obsidian.vault_path` and `obsidian.vault_name` in config.

## Subcommands

### `overseer note daily`

Open or create today's daily note in Obsidian. Uses the configured `obsidian.daily_notes_folder`. If the note doesn't exist, it is created from the configured daily template.

```bash
overseer note daily
```

### `overseer note new`

Create a new note. Presents an interactive folder and template picker, then opens the new note in Obsidian.

```bash
overseer note new
overseer note new "Meeting notes"   # skip the title prompt
```

### `overseer note search`

Search all markdown files in the vault for a query string. Returns matching lines with file path and line number.

```bash
overseer note search kubernetes
overseer note search kubernetes --format json
```

JSON output:

```json
[
  {
    "file": "Daily/2025-04-10.md",
    "line_num": 12,
    "line": "- Kubernetes context switching with kubectx"
  }
]
```

## Config

| Key | Description |
|---|---|
| `obsidian.vault_path` | Absolute path to the vault |
| `obsidian.vault_name` | Vault name as registered in Obsidian |
| `obsidian.daily_notes_folder` | Folder for daily notes |
| `obsidian.templates_folder` | Folder for note templates |
| `obsidian.default_folder` | Default folder for `note new` |
