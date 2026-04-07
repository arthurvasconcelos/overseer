# Overseer Design System

Reference for plugin authors. The canonical implementation lives in
[cli/internal/tui/styles.go](../cli/internal/tui/styles.go) — this document
is the source of truth for porting the palette to other languages.

## Palette

All colours use the xterm 256-colour index, which works in any modern terminal.

| Token    | Code | Colour  | Usage                              | Bold |
|----------|------|---------|------------------------------------|------|
| Header   | 99   | Purple  | Section titles                     | Yes  |
| Accent   | 212  | Pink    | Keys, channels, usernames          | No   |
| OK       | 82   | Green   | Success states                     | Yes  |
| Warn     | 214  | Amber   | Warnings                           | Yes  |
| Error    | 196  | Red     | Errors                             | Yes  |
| Muted    | 240  | Dark grey | Hints, empty state, secondary badges | No |
| Dim      | 245  | Grey    | Secondary info                     | No   |
| Normal   | 252  | Light   | Body text                          | No   |

## Common patterns

### Section header

```
▸ Label  ·  badge text
```

- `▸ Label` — Header style
- `  ·  badge text` — Muted style
- Two spaces before and after `·`

### Warning line

```
⚠  label: message
```

- `⚠  label:` — Warn style
- ` message` — Muted style
- Two spaces after `⚠`

## SDK implementations

| Language   | Library | Location               |
|------------|---------|------------------------|
| Go         | lipgloss | `cli/internal/tui/styles.go` |
| Python     | rich    | `sdk/python/overseer_sdk/styles.py` |
| TypeScript | chalk   | `sdk/typescript/src/styles.ts` |

When adding a new language, use the colour codes from the palette table above
and mirror the `section_header` and `warn_line` helpers.
