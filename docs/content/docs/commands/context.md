---
title: context
weight: 6
---

Print a self-contained description of overseer for use in AI assistant chats.

```bash
overseer context
overseer context --format json
```

## Text output

Generates a comprehensive markdown document describing all commands, flags, config structure, and key concepts. Designed to be pasted directly into an AI chat as context.

```bash
# Copy to clipboard (macOS)
overseer context | pbcopy

# Save to a file
overseer context > overseer-context.md
```

## JSON output

Returns a structured command manifest with the tool name, version, description, and a full tree of commands with their flags and subcommands.

```json
{
  "tool": "overseer",
  "version": "1.2.3",
  "description": "...",
  "brain": "...",
  "commands": [
    {
      "path": "overseer brain",
      "short": "Manage the brain directory",
      "subcommands": [...]
    }
  ]
}
```

## MCP alternative

For persistent AI integration rather than a one-off paste, see [`overseer mcp`](/docs/commands/mcp).
