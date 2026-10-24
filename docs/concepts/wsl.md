---
description: Run overseer under Windows Subsystem for Linux with 1Password on the Windows side — op.exe, ssh.exe, and the WSL commit signer.
outline: deep
---

# WSL

overseer runs under WSL like any Linux install, but 1Password lives on the Windows side. The 1Password CLI, SSH agent and commit signer all have to be reached through their Windows binaries. This page covers the setup and the errors you hit when one piece is missing.

## Install

Install [Homebrew on Linux](https://docs.brew.sh/Homebrew-on-Linux) inside WSL, then:

```bash
brew install arthurvasconcelos/tap/overseer
```

## Bringing over an existing brain

Set up the 1Password CLI and `ssh.exe` below first, then run `overseer setup` and pick **clone existing**. If `core.sshCommand` isn't set when you clone over SSH, setup offers to set it to `ssh.exe`.

Afterwards, `overseer brain status` lists the macOS-only paths in your shared config and dotfiles. Move those into `config.wsl.yaml` / `dotfiles.wsl/` (or `config.darwin.yaml` / `dotfiles.darwin/` for the macOS side). See [Brain → Platforms](/concepts/brain#platforms).

## 1Password CLI: use `op.exe`

The 1Password desktop app integration only works through the **Windows** CLI, `op.exe`. Install 1Password and its CLI on Windows, turn on **Settings → Developer → Integrate with 1Password CLI**, and alias it in your WSL shell:

```bash
echo 'alias op="op.exe"' >> ~/.zshrc   # or ~/.bashrc
```

Check it with `op vault list`. It should show the 1Password approval prompt on Windows.

**Don't install the Linux `op` in WSL.** It has no access to the desktop app. A shell alias only applies to your interactive shell, so any program that runs `op` directly (scripts, git hooks, other tools) gets the Linux binary instead.

overseer doesn't rely on the alias. Under WSL it calls `op.exe` directly whenever it is on `PATH` (since v0.27.2), even if a Linux `op` is also installed. If you installed the Linux one by mistake, remove it anyway: `sudo apt remove 1password-cli` or `brew uninstall 1password-cli` (`which -a op` shows where it is).

## SSH: use `ssh.exe`

Git in WSL uses the Linux `ssh`, which can't reach the 1Password SSH agent. Point git at the Windows client:

```bash
git config --global core.sshCommand ssh.exe
```

This covers every git operation over SSH, including `overseer brain push`, `brain pull` and `repos pull`. overseer never overwrites `core.sshCommand`. It only offers to set it, when it's unset, before cloning a brain over SSH. Applying a git profile only sets `user.*`, `gpg.*` and `commit.gpgsign`.

If your brain has a `.gitconfig` dotfile, add the setting to a WSL-only copy at `dotfiles.wsl/.gitconfig` instead, so it survives `overseer brain setup` without affecting macOS.

## Commit signing: use `op-ssh-sign-wsl.exe`

To sign commits, `gpg_ssh_program` must point at 1Password's **WSL signer**. Find it with:

```bash
which op-ssh-sign-wsl.exe
```

Use the path this prints. Since 1Password 8.11.18 it's `/mnt/c/Users/<you>/AppData/Local/Microsoft/WindowsApps/op-ssh-sign-wsl.exe`. That folder is usually on the `PATH` WSL inherits from Windows.

::: danger Not `ssh.exe`
`which ssh.exe` prints a similar-looking path (`/mnt/c/WINDOWS/System32/OpenSSH/ssh.exe`), but that's the SSH **client** and belongs in `core.sshCommand`. Used as `gpg_ssh_program`, every commit fails with `ssh: Could not resolve hostname sign`.
:::

If `which` prints nothing:

- Check the folder directly: `ls /mnt/c/Users/*/AppData/Local/Microsoft/WindowsApps/op-ssh-sign-wsl.exe`. If the file is there, Windows `PATH` isn't being passed into WSL; look for `appendWindowsPath = false` in `/etc/wsl.conf`. The full path still works for `gpg_ssh_program`.
- Installs older than 8.11.18 put the signer at `/mnt/c/Users/<you>/AppData/Local/1Password/app/8/op-ssh-sign-wsl` (no `.exe`). Update 1Password, or use that path.
- In the 1Password app, **Settings → Developer → Configure Commit Signing** shows the path for your install.

Since v0.27.3, `overseer setup` and `overseer init` run the same lookup and suggest the result as the default. To set it by hand, paste the path into the machine-local config, `~/.config/overseer/config.local.yaml`:

```yaml
system:
  gpg_ssh_program: /mnt/c/Users/<you>/AppData/Local/Microsoft/WindowsApps/op-ssh-sign-wsl.exe
```

Changing the config doesn't update repos that already have a profile applied, because the old value stays in their `.git/config`. `overseer brain push` re-applies the brain's profile on every run. For other repos, run `overseer git profile apply <name>` inside the repo, then check it:

```bash
git config gpg.ssh.program   # should print the op-ssh-sign-wsl.exe path
```

::: warning Keep the signer path out of the shared config
For `gpg_ssh_program`, the profile value wins over `git.defaults`, which wins over `system`. If your brain's shared `config.yaml` sets `gpg_ssh_program` under `git.defaults` or on a profile (say, the macOS `op-ssh-sign` path), it overrides the WSL path in `config.local.yaml`. `overseer setup` warns when that happens. Move the value to `config.darwin.yaml` and put the WSL signer in `config.wsl.yaml`, or set it only in each machine's `config.local.yaml`.
:::

## Troubleshooting

| Error | Cause | Fix |
|---|---|---|
| `No accounts configured for use with 1Password CLI` | The Linux `op` ran instead of `op.exe` (overseer before v0.27.2, or another tool) | Upgrade overseer; remove the Linux `op` |
| `ssh: Could not resolve hostname sign: No such host is known.` on commit | `gpg_ssh_program` points at `ssh.exe`, which reads the `-Y sign` arguments as a hostname | Set `gpg_ssh_program` to `op-ssh-sign-wsl.exe`; `ssh.exe` belongs in `core.sshCommand` |
| `Permission denied (publickey)` or a hang on push/pull | Git is using the Linux `ssh` | `git config --global core.sshCommand ssh.exe` |
| `The current branch master has no upstream branch` on `brain push` | overseer before v0.27.4 didn't handle the first push to a new repo | Upgrade overseer, or run `git -C <brain> push --set-upstream origin master` once |
