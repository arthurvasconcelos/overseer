---
title: Brain
weight: 1
---

The **brain** is a private git repo that holds your personal config. It is intentionally separate from the overseer binary — your config can be version-controlled, backed up, and shared across machines without touching the tool itself.

## Structure

```
brain/
  overseer/
    config.yaml          # integrations, git profiles, repos, etc.
    Brewfile             # Homebrew packages
    Brewfile.local       # machine-specific packages (gitignored)
    dotfiles/            # mirrors ~/ — symlinked by overseer brain setup
      .zshrc             → ~/.zshrc
      .gitconfig         → ~/.gitconfig
      .config/
        starship.toml    → ~/.config/starship.toml
    plugins/             # overseer-* plugin executables
```

See [brain-example/](https://github.com/arthurvasconcelos/overseer/tree/main/brain-example) in the repo for a fully commented example layout.

## Path resolution

overseer finds your brain in this order:

1. `OVERSEER_BRAIN` environment variable
2. `system.brain_path` in `~/.config/overseer/config.local.yaml`
3. `brain.path` in `brain/overseer/config.yaml`
4. `~/brain` (default)

Print the resolved path at any time:

```bash
overseer brain path
```

## Referenced paths

Not everything overseer uses lives in the brain. Some directories are **referenced by path** in `config.yaml` and can stay wherever they already are:

| Config key | What it points to |
|---|---|
| `obsidian.vault_path` | Your Obsidian vault |
| `system.repos_path` (local) | Where managed repos are cloned |

These are typically machine-specific and belong in `config.local.yaml`, not the shared brain config.

## Brain commands

| Command | Description |
|---|---|
| `overseer brain init` | Clone the brain repo from `brain.url` |
| `overseer brain setup` | Pull latest changes and re-apply config |
| `overseer brain status` | Show brain git status and health |
| `overseer brain pull` | `git pull` the brain repo |
| `overseer brain push` | `git push` the brain repo |
| `overseer brain git-init` | Initialise a new git repo in an existing brain directory |
| `overseer brain path` | Print the resolved brain path |
