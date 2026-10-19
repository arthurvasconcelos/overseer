# setup


Interactive bootstrap wizard for a new machine. Walks through everything in one session: brain path, git remote, machine settings, directory scaffolding, dotfile wiring, and Brew packages.

```bash
overseer setup
overseer setup --dry-run   # preview without making changes
```

Safe to re-run at any time — existing values are shown as defaults and nothing is overwritten without your input.

## What it does

1. **Brain**: prompts for the brain path and its git remote URL.
2. **Machine**: prompts for `repos_path` and `gpg_ssh_program`. The signer default depends on the platform: 1Password's `op-ssh-sign` on macOS and Linux, `op-ssh-sign-wsl.exe` under [WSL](/concepts/wsl).
3. **Git**: optionally sets git defaults (user name, signing format) and creates a first git profile. Skipped when profiles already exist.
4. Writes the machine-local `config.local.yaml` (see [Config](/concepts/config#two-config-files) for its location).
5. **Scaffold**: creates `overseer/dotfiles/`, `overseer/plugins/`, a `Brewfile` and a commented `config.yaml` in the brain if they're missing, then saves the git defaults, first profile and remote URL into `config.yaml`. The template's comments are kept.
6. **Version control**: optionally turns the brain into a git repo, either a new repo or a clone of an existing remote.
7. **Dotfiles**: symlinks `brain/overseer/dotfiles/` into `~/`.
8. **Packages** (macOS only): runs `brew bundle` from the Brewfile.

To re-apply config without the interactive wizard, use [`overseer brain setup`](/commands/brain#overseer-brain-setup).
