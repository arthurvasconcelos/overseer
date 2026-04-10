---
title: brain
weight: 2
---

Manage the brain directory. See [Concepts → Brain](/docs/concepts/brain) for what the brain is.

## Subcommands

### `overseer brain init`

Clone the brain repo from `brain.url` in config. Use this on a fresh machine to pull down your existing brain.

```bash
overseer brain init
```

### `overseer brain setup`

Pull the latest brain changes and re-apply config: symlinks dotfiles, installs Brew packages, and applies git profiles. Equivalent to running the relevant parts of the setup wizard without the interactive prompts.

```bash
overseer brain setup
overseer brain setup --dry-run   # preview without making changes
```

### `overseer brain pull`

Run `git pull` in the brain directory.

```bash
overseer brain pull
```

### `overseer brain push`

Run `git push` in the brain directory.

```bash
overseer brain push
```

### `overseer brain git-init`

Initialise a new git repo in an existing brain directory. Use this when setting up a brain for the first time without an existing remote.

```bash
overseer brain git-init
```

### `overseer brain status`

Show brain directory health: git status, last commit, and whether dotfile symlinks are in place.

```bash
overseer brain status
```

### `overseer brain path`

Print the resolved brain path. Useful for scripts that need to reference the brain directory.

```bash
overseer brain path
# → /Users/you/brain
```
