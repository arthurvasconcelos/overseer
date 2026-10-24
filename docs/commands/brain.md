# brain


Manage the brain directory. See [Concepts → Brain](/concepts/brain) for what the brain is.

## Subcommands

### `overseer brain clone`

Clone an existing brain onto this machine, into the resolved brain path, then apply `brain.git_profile` to it. The URL defaults to `brain.url` in config.

```bash
overseer brain clone git@github.com:you/brain.git
```

- The brain path must be empty or missing. If it's already a git repository, nothing is changed.
- Under [WSL](/concepts/wsl), cloning over SSH first offers to set `core.sshCommand` to `ssh.exe` if it isn't set.

Run `overseer setup` afterwards for machine settings and dotfiles. `overseer setup` can also do the clone itself.

### `overseer brain setup`

Re-apply the brain without the wizard: symlinks dotfiles (`dotfiles/` plus the `dotfiles.<platform>/` overlays, see [Brain → Platforms](/concepts/brain#platforms)) and installs Brew packages on macOS.

- Existing files are moved to `~/.overseer-backups/<timestamp>/` before being replaced, under the same path they had in your home folder.
- Links to brain files that don't apply on this platform any more are removed, and the latest backup of the file they replaced is moved back into place.

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

Commit every change in the brain and push it.

```bash
overseer brain push
```

- Applies `brain.git_profile` (if set) to the brain repo, then commits all changes with a message listing the affected files and the machine they came from.
- On the first push to a new, empty remote it sets the upstream (`git push --set-upstream origin HEAD`). If there is no `origin` remote yet, it adds one from `brain.url`.
- If there's nothing new to commit but earlier local commits never reached the remote (say, a previous push failed), it pushes those.
- Prints "nothing to push, brain is up to date" when there's nothing to commit or push.

Git does the SSH and commit signing using your git config and the profile's `gpg_ssh_program`. Under WSL both need the Windows binaries. See [WSL](/concepts/wsl).

### `overseer brain git-init`

Initialise a new git repo in an existing brain directory. Use this when setting up a brain for the first time without an existing remote.

```bash
overseer brain git-init
```

### `overseer brain status`

Show brain directory health: git status, platform overlays in use, whether dotfile symlinks are in place, and shared content that only works on some platforms (absolute paths in `config.yaml`, platform-specific paths in `dotfiles/`).

```bash
overseer brain status
overseer brain status --fix   # repair what can be repaired automatically, then report the rest
```

`--fix` edits the shared `config.yaml` line by line, keeping comments and formatting:

- Absolute paths in your home folder become `~/...`. Paths under `/Users/<you>` or `/home/<you>` count too, so a path written on another platform is caught.
- `git.defaults.gpg_ssh_program` moves to the `config.<platform>.yaml` its value belongs to (`/Applications/...` → `darwin`, `.exe` or `/mnt/...` → `wsl`). An overlay that already sets it keeps its own value.

Everything else stays listed for you to fix by hand: other absolute paths, signers set on a profile (overlays replace the whole profiles list, so they can't be moved one at a time), and dotfiles. Review the result with `git -C "$(overseer brain path)" diff` before pushing.

### `overseer brain path`

Print the resolved brain path. Useful for scripts that need to reference the brain directory.

```bash
overseer brain path
# → /Users/you/brain
```
