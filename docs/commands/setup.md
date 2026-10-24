# setup


Interactive bootstrap wizard for a new machine. Walks through everything in one session: getting the brain (clone, new or existing), machine settings, git profiles, dotfile wiring, and Brew packages.

```bash
overseer setup
overseer setup --dry-run   # preview without making changes
```

Safe to re-run at any time — existing values are shown as defaults and nothing is overwritten without your input.

## What it does

1. **Brain**: if a brain already exists at the resolved path, it is used. Otherwise you choose:
   - **clone existing**: you already have a brain repository. Prompts for the brain path and remote URL.
   - **start new**: scaffold a fresh brain. Prompts for the brain path and an optional remote URL.
   - **already on disk**: a brain exists in another directory. Prompts for its path.
2. **Machine**: prompts for `repos_path` and `gpg_ssh_program`. The signer default depends on the platform: 1Password's `op-ssh-sign` on macOS and Linux, `op-ssh-sign-wsl.exe` under [WSL](/concepts/wsl).
3. Writes the machine-local `config.local.yaml` (see [Config](/concepts/config#two-config-files) for its location).
4. **Clone** (clone existing only): clones the brain into an empty or missing directory. Under WSL it first offers to set `core.sshCommand` to `ssh.exe`. Config is then reloaded from the cloned brain, including its [platform overlays](/concepts/config#layers). Setup warns if the brain's `git.defaults.gpg_ssh_program` doesn't exist on this machine.
5. **Scaffold** (start new only): creates `overseer/dotfiles/`, `overseer/plugins/`, a `Brewfile` and a commented `config.yaml` if they're missing, and saves the remote URL into `config.yaml`.
6. **Git**: optionally sets git defaults (user name, signing format) and creates a first git profile, saved into the brain's `config.yaml`. Skipped when the brain already has profiles.
7. **Version control**: applies `brain.git_profile` if the brain is a git repo; otherwise offers to make it one.
8. **Dotfiles**: symlinks `dotfiles/` and the `dotfiles.<platform>/` overlays into `~/`.
9. **Packages** (macOS only): runs `brew bundle` from the Brewfile.

To re-apply config without the interactive wizard, use [`overseer brain setup`](/commands/brain#overseer-brain-setup).
