---
title: ssh
weight: 16
---

Manage SSH config profiles. Profiles are named SSH config blocks stored in the brain. Activating a profile writes it to `~/.ssh/overseer_active.conf`, which is included by `~/.ssh/config` via an overseer-managed `Include` directive.

```bash
overseer ssh list            # list all profiles
overseer ssh show <name>     # print the SSH config block for a profile
overseer ssh use <name>      # activate a profile
overseer ssh setup           # add the Include directive to ~/.ssh/config (one-time setup)
```

## First-time setup

Run `overseer ssh setup` once to add the overseer `Include` directive to `~/.ssh/config`. This is idempotent — safe to run again.

```bash
overseer ssh setup
```

After that, switching profiles is a single command:

```bash
overseer ssh use work       # activates the "work" SSH profile
overseer ssh use personal   # switches to the "personal" profile
```

## Config

SSH profiles are stored in the brain under `overseer/ssh/`. Each file is a standard SSH config block named `<profile>.conf`:

```
# brain/overseer/ssh/work.conf
Host github.com
  IdentityFile ~/.ssh/id_work
  User git
```
