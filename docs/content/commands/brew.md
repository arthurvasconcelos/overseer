---
title: brew
weight: 3
---

Manage Homebrew packages via a Brewfile in your brain.

The Brewfile path is configured via `brew.brewfile` in `config.yaml` (relative to `system.repos_path`). Defaults to `overseer/Brewfile` inside your repos directory.

## Subcommands

### `overseer brew check`

Show packages listed in the Brewfile that are not yet installed on this machine.

```bash
overseer brew check
```

### `overseer brew install`

Run `brew bundle` to install all packages from the Brewfile.

```bash
overseer brew install
```

### `overseer brew dump`

Update the Brewfile from currently installed packages (`brew bundle dump --force`).

```bash
overseer brew dump
```
