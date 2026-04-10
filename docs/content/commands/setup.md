---
title: setup
weight: 15
---

Interactive bootstrap wizard for a new machine. Walks through everything in one session: brain path, git remote, machine settings, directory scaffolding, dotfile wiring, and Brew packages.

```bash
overseer setup
overseer setup --dry-run   # preview without making changes
```

Safe to re-run at any time — existing values are shown as defaults and nothing is overwritten without your input.

## What it does

1. Resolves (or prompts for) the brain path
2. Clones the brain repo if a remote is configured and the brain isn't present
3. Applies dotfile symlinks from `brain/overseer/dotfiles/`
4. Runs `brew bundle` from the Brewfile
5. Applies git profiles to managed repos

To re-apply config without the interactive wizard, use [`overseer brain setup`](/commands/brain#overseer-brain-setup).
