---
description: Capture structured learning entries, review due items, and query the learning database.
---

# overseer learn

Capture and review structured learning entries stored in the brain at `overseer/learning.db`.

## Commands

| Command | Description |
|---|---|
| `overseer learn add [topic]` | Add a learning entry. Prompts for missing topic, source, description, and quiz questions. |
| `overseer learn due` | Show entries due today or earlier. |
| `overseer learn review` | Run an interactive review for due entries, or review a specific item with flags. |
| `overseer learn status` | Show active entry count, due count, upcoming reviews, and recent review count. |
| `overseer learn search <query>` | Search topics, sources, descriptions, and quiz questions. |
| `overseer learn show <entry-id>` | Show one learning entry with quiz questions, schedule, review summary, and review history. |
| `overseer learn archive <entry-id>` | Archive an active learning entry. |

## JSON output

Structured commands support the global `--format json` flag:

```sh
overseer learn due --format json
overseer learn status --format json
overseer learn search sqlite --format json
overseer learn show 1 --format json
overseer learn archive 1 --format json
```

## Non-interactive add

```sh
overseer learn add "SQLite indexes" \
  --source "database notes" \
  --description "Indexes speed reads by adding write and storage cost." \
  --quiz "What do indexes trade for faster lookups?"
```

Active topics must be unique by normalized topic match. Normalization ignores casing, repeated whitespace, and simple punctuation. Use `--allow-duplicate` to bypass that guard.

## Future extensions

Planned extensions include YAML import, Obsidian concept-note export, MCP draft/approve flow, tags and projects, backlinks/source anchors, richer review analytics, and smarter scheduling.
