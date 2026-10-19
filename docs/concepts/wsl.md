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

This covers every git operation over SSH, including `overseer brain push`, `brain pull` and `repos pull`. overseer never overwrites `core.sshCommand`. Applying a git profile only sets `user.*`, `gpg.*` and `commit.gpgsign`.

## Commit signing: use `op-ssh-sign-wsl.exe`

To sign commits, `gpg_ssh_program` must point at 1Password's **WSL signer**, not at `ssh.exe`:

```bash
which op-ssh-sign-wsl.exe
```

Since 1Password 8.11.18 the signer is at `/mnt/c/Users/<you>/AppData/Local/Microsoft/WindowsApps/op-ssh-sign-wsl.exe`. That folder is usually on the `PATH` WSL inherits from Windows. Older installs use `/mnt/c/Users/<you>/AppData/Local/1Password/app/8/op-ssh-sign-wsl`. In the 1Password app, **Settings → Developer → Configure Commit Signing** shows the right path for your install.

Since v0.27.3, `overseer setup` and `overseer init` find it and suggest it as the default. To set it by hand, put it in the machine-local config, `~/.config/overseer/config.local.yaml`:

```yaml
system:
  gpg_ssh_program: /mnt/c/Users/<you>/AppData/Local/Microsoft/WindowsApps/op-ssh-sign-wsl.exe
```

::: warning Keep the signer path machine-local
For `gpg_ssh_program`, the profile value wins over `git.defaults`, which wins over `system`. If your brain's `config.yaml` sets `gpg_ssh_program` under `git.defaults` or on a profile (say, the macOS `op-ssh-sign` path), it overrides the WSL path in `config.local.yaml`. If the same brain is used on different platforms, set `gpg_ssh_program` only in each machine's `config.local.yaml`.
:::

## Troubleshooting

| Error | Cause | Fix |
|---|---|---|
| `No accounts configured for use with 1Password CLI` | The Linux `op` ran instead of `op.exe` (overseer before v0.27.2, or another tool) | Upgrade overseer; remove the Linux `op` |
| `ssh: Could not resolve hostname sign: No such host is known.` on commit | `gpg_ssh_program` points at `ssh.exe`, which reads the `-Y sign` arguments as a hostname | Set `gpg_ssh_program` to `op-ssh-sign-wsl.exe`; `ssh.exe` belongs in `core.sshCommand` |
| `Permission denied (publickey)` or a hang on push/pull | Git is using the Linux `ssh` | `git config --global core.sshCommand ssh.exe` |
| `The current branch master has no upstream branch` on `brain push` | overseer before v0.27.4 didn't handle the first push to a new repo | Upgrade overseer, or run `git -C <brain> push --set-upstream origin master` once |
