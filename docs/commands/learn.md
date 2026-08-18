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
| `overseer learn edit <entry-id>` | Correct an entry in place, keeping its id, review history, and schedule. |
| `overseer learn archive <entry-id>` | Archive an active learning entry. |

## JSON output

Structured commands support the global `--format json` flag:

```sh
overseer learn due --format json
overseer learn status --format json
overseer learn search sqlite --format json
overseer learn show 1 --format json
overseer learn edit 1 --description "Corrected." --format json
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

## Correcting an entry

An entry captured with a factual error is worse than no entry at all: spaced repetition will
rehearse the mistake until it sticks. Fix it in place rather than archiving and recapturing, so
the id, review history, and schedule survive.

```sh
overseer learn edit 49 \
  --description "The mask window ramps from 180°, peaks at 252°, and is gone by 320°." \
  --note "the ~68° figure read only the plateau and missed the ramp"
```

Only the flags you pass are changed. `--quiz` replaces every question, so pass it once per question
you want to keep. `--source ""` clears the source. With no field flags, the current values are
offered for editing interactively; the description opens in a multi-line field where `ctrl+e` hands
off to `$EDITOR`.

Each edit stamps `corrected_at` and stores the `--note` as a revision note. When an entry has been
corrected since you last reviewed it, the next review leads with that note, so a correction is
itself surfaced rather than silently swapped in.

## Future extensions

Planned extensions include YAML import, Obsidian concept-note export, tags and projects,
backlinks/source anchors, richer review analytics, and smarter scheduling.
