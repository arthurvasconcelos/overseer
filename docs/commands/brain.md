# brain


Manage the brain directory. See [Concepts → Brain](/concepts/brain) for what the brain is.

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
