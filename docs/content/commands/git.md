---
title: git
weight: 8
---

Git identity management. Define named profiles with different email addresses, signing keys, and git settings, then apply them per-repo or globally.

## `overseer git setup`

Interactive profile picker. Presents all configured profiles and applies the selected one to the current repo (or globally with `--global`).

```bash
overseer git setup
overseer git setup --global
```

## `overseer git profile`

Manage git profiles.

### list

```bash
overseer git profile list
overseer git profile list --format json
```

JSON output: array of profile objects.

```json
[
  {
    "name": "personal",
    "email": "you@personal.com",
    "signing_key": "op://Personal/SSH Key/public key",
    "user_name": "Arthur Vasconcelos",
    "gpg_format": "ssh",
    "gpg_ssh_program": "/Applications/1Password.app/Contents/MacOS/op-ssh-sign",
    "commit_gpgsign": true,
    "op_account": ""
  }
]
```

### add

Interactively create a new git profile and save it to config.

```bash
overseer git profile add
```

### edit

Edit an existing profile.

```bash
overseer git profile edit personal
overseer git profile edit          # interactive picker
```

### remove

Remove a profile from config.

```bash
overseer git profile remove personal
```

### apply

Apply a profile to the current repo (or globally).

```bash
overseer git profile apply personal
overseer git profile apply personal --global
```

This sets `user.email`, `user.name`, `user.signingKey`, `gpg.format`, `gpg.ssh.program`, and `commit.gpgSign` in `.git/config` (or `~/.gitconfig` with `--global`).

### defaults

View and edit shared git defaults that are merged into every profile.

```bash
overseer git profile defaults
```

## Config

Profiles are stored in `git.profiles[]` and shared defaults in `git.defaults`. See [Concepts → Config](/concepts/config#git) for the full schema.
