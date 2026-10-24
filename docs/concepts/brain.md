---
description: The brain is a private git repo that holds your overseer config, dotfiles, and personal data — separate from the tool itself.
outline: deep
---

# Brain


The **brain** is a private git repo that holds your personal config. It is intentionally separate from the overseer binary — your config can be version-controlled, backed up, and shared across machines without touching the tool itself.

## Structure

```
brain/
  overseer/
    config.yaml          # integrations, git profiles, repos, etc.
    config.darwin.yaml   # optional per-platform overlays (see Platforms below)
    config.wsl.yaml
    Brewfile             # Homebrew packages
    Brewfile.local       # machine-specific packages (gitignored)
    dotfiles/            # mirrors ~/ — symlinked by overseer brain setup
      .zshrc             → ~/.zshrc
      .gitconfig         → ~/.gitconfig
      .config/
        starship.toml    → ~/.config/starship.toml
    dotfiles.wsl/        # optional per-platform dotfiles, replace dotfiles/ entries
      .gitconfig         → ~/.gitconfig on WSL only
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

## Using an existing brain on a new machine

Run the setup wizard and pick **clone existing** when it asks where the brain comes from:

```bash
overseer setup
```

It writes `config.local.yaml`, clones the brain, then reloads config from it. Your git profiles, `brain.git_profile` and dotfiles come from the brain you cloned, so setup doesn't ask you to create them again. The brain path must be empty or missing; setup never clones over existing files.

To clone without the wizard, use `overseer brain clone [url]` (the URL defaults to `brain.url`), then run `overseer setup` or `overseer brain setup`.

Under [WSL](/concepts/wsl), cloning over SSH first offers to set `core.sshCommand` to `ssh.exe` if it isn't set, so the 1Password SSH agent is reachable.

## Platforms

One brain can serve macOS, Linux and WSL. Keep shared values in `config.yaml` and put what differs per platform in overlays next to it:

| File | Loaded on |
|---|---|
| `config.darwin.yaml` | macOS |
| `config.linux.yaml` | Linux and WSL |
| `config.wsl.yaml` | WSL, after `config.linux.yaml` |

Dotfiles follow the same rule: a file in `dotfiles.<platform>/` replaces the file at the same path in `dotfiles/`. For example, `dotfiles.wsl/.gitconfig` can set `core.sshCommand = ssh.exe` while macOS keeps the shared `.gitconfig`. When the overlay changes, `overseer brain setup` repoints the links.

Only files that work everywhere belong in `dotfiles/`, because a file there is linked on **every** platform that doesn't have its own copy. Shell startup files (`.zshrc`, `.bashrc`) rarely qualify: they load tools from platform-specific places. Keep one per platform instead:

```
dotfiles.darwin/.zshrc   # macOS
dotfiles.linux/.zshrc    # Linux, and WSL unless dotfiles.wsl/ has one
dotfiles.wsl/.zshrc      # WSL only
```

A platform with no copy gets no link, and keeps whatever file it already has.

When a file stops applying on a platform (for example, you move `dotfiles/.zshrc` to `dotfiles.darwin/.zshrc`), `overseer brain setup` on the other platforms removes the link and moves the file it had replaced back from `~/.overseer-backups/`.

See [Config → Layers](/concepts/config#layers) for the merge order.

## Paths

Paths in the brain should work on every machine:

- Write them relative to your home folder: `~/repos/notes`, not `/Users/you/repos/notes`. `~`, `$HOME` and other `$VARS` are expanded in every path setting.
- `obsidian.vault_path`, `repos[].path` and `brew.brewfile` can also be relative; they resolve against `system.repos_path`.
- A path that really differs per platform (say, a vault on the Windows drive under WSL) goes in `config.<platform>.yaml`.

`overseer claude skills add-path` and `overseer codex skills add-path` store paths in your home folder as `~/...` automatically.

`overseer brain status` lists absolute paths in the shared `config.yaml`, and lines in the shared `dotfiles/` that mention a platform-specific path (`/Users/`, `/Applications/`, `/opt/homebrew/`, `/home/`, `/mnt/c/`, ...).

### Moving an existing brain over

Brains set up before platform overlays existed keep working unchanged: with no `config.<platform>.yaml` and no `dotfiles.<platform>/`, nothing behaves differently. To make one portable:

1. Run `overseer brain status --fix`. It rewrites home paths as `~/...` and moves `git.defaults.gpg_ssh_program` into its platform overlay. See [`brain status`](/commands/brain#overseer-brain-status).
2. Fix what it still lists. For a dotfile that differs only in a few settings, keep it shared and include a per-platform file. For git:

   ```ini [dotfiles/.gitconfig]
   [include]
       path = ~/.gitconfig.platform
   ```

   Then put the signer in `dotfiles.darwin/.gitconfig.platform` and `dotfiles.wsl/.gitconfig.platform`. Git skips an include whose file is missing.
3. Run `overseer brain setup` on each machine to link the new overlay files, then `overseer brain push`.

## Brain commands

| Command | Description |
|---|---|
| `overseer brain clone [url]` | Clone an existing brain onto this machine |
| `overseer brain setup` | Wire dotfiles and install Brew packages |
| `overseer brain status` | Show brain git status and health |
| `overseer brain pull` | `git pull` the brain repo |
| `overseer brain push` | `git push` the brain repo |
| `overseer brain git-init` | Initialise a new git repo in an existing brain directory |
| `overseer brain path` | Print the resolved brain path |
