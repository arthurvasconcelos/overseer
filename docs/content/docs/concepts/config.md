---
title: Config
weight: 2
---

## Two config files

overseer merges two config files on every run. The local file always takes precedence:

| File | Purpose |
|---|---|
| `brain/overseer/config.yaml` | Shared, portable, committed to your brain repo |
| `~/.config/overseer/config.local.yaml` | Machine-local overrides — tokens, paths, never committed |

Inspect the merged result at any time:

```bash
overseer config           # human-readable summary
overseer config --format json  # full config as JSON
```

## JSON Schema

The full JSON Schema for `config.yaml` is available via:

```bash
overseer config schema
```

You can also point your editor at the schema for inline validation and autocomplete:

```yaml
# brain/overseer/config.yaml
# yaml-language-server: $schema=https://arthurvasconcelos.github.io/overseer/schema.json
```

## Config keys

### `secrets`

```yaml
secrets:
  environments:
    p24: "he7em6mxcdlsewgnzrqldjizi"  # alias → 1Password account ID
```

| Key | Description |
|---|---|
| `secrets.environments` | Map of alias → 1Password account ID. Used with `overseer run --env <alias>`. |

---

### `integrations`

```yaml
integrations:
  jira:
    - name: work
      base_url: https://company.atlassian.net
      email: you@company.com
      token: "op://Work/Jira/token"
      op_account: work
  slack:
    - name: work
      token: "op://Work/Slack/token"
  google:
    - name: personal
      credentials_doc: "op://Personal/Google/credentials_json"
  github:
    - name: personal
      token: "op://Personal/GitHub/token"
  gitlab:
    - name: work
      base_url: https://gitlab.company.com
      token: "op://Work/GitLab/token"
```

| Key | Fields |
|---|---|
| `integrations.jira[]` | `name`, `base_url`, `email`, `token`, `op_account` |
| `integrations.slack[]` | `name`, `token`, `op_account` |
| `integrations.google[]` | `name`, `credentials_doc`, `op_account` |
| `integrations.github[]` | `name`, `token`, `op_account` |
| `integrations.gitlab[]` | `name`, `base_url`, `token`, `op_account` |

---

### `git`

```yaml
git:
  defaults:
    user_name: Arthur Vasconcelos
    gpg_format: ssh
    gpg_ssh_program: /Applications/1Password.app/Contents/MacOS/op-ssh-sign
    commit_gpgsign: true
  profiles:
    - name: personal
      email: arthur@personal.com
      signing_key: "op://Personal/SSH Key/public key"
    - name: work
      email: arthur@company.com
      signing_key: "op://Work/SSH Key/public key"
      op_account: work
```

| Key | Description |
|---|---|
| `git.defaults` | Shared settings applied to all profiles unless overridden |
| `git.profiles[]` | Named git identities. Fields: `name`, `email`, `signing_key`, `user_name`, `gpg_format`, `gpg_ssh_program`, `commit_gpgsign`, `op_account` |

---

### `system` (machine-local)

```yaml
system:
  repos_path: ~/repos
  brain_path: ~/brain
```

| Key | Description |
|---|---|
| `system.repos_path` | Where managed repos are cloned |
| `system.brain_path` | Override brain directory path for this machine |

These belong in `config.local.yaml`, not the shared brain config.

---

### `brain`

```yaml
brain:
  path: ~/brain
  url: git@github.com:you/brain.git
  git_profile: personal
```

| Key | Description |
|---|---|
| `brain.path` | Canonical brain path (portable across machines) |
| `brain.url` | Git remote for cloning and pushing |
| `brain.git_profile` | Git profile to use for brain commits |

---

### `obsidian`

```yaml
obsidian:
  vault_path: ~/Documents/Notes
  vault_name: Notes
  daily_notes_folder: Daily
  templates_folder: Templates
  default_folder: Inbox
```

| Key | Description |
|---|---|
| `obsidian.vault_path` | Absolute path to your Obsidian vault |
| `obsidian.vault_name` | Vault name as registered in Obsidian |
| `obsidian.daily_notes_folder` | Folder for daily notes |
| `obsidian.templates_folder` | Folder for note templates |
| `obsidian.default_folder` | Default folder for new notes |

---

### `brew`

```yaml
brew:
  brewfile: overseer/Brewfile
```

| Key | Description |
|---|---|
| `brew.brewfile` | Brewfile path relative to `repos_path` |

---

### `repos`

```yaml
repos:
  - name: brain
    url: git@github.com:you/brain.git
    path: ~/brain
    git_profile: personal
  - name: work-api
    url: git@gitlab.company.com:team/api.git
    path: ~/repos/work/api
    git_profile: work
    readonly: false
```

| Field | Description |
|---|---|
| `name` | Display name |
| `url` | Git remote URL |
| `path` | Local path (absolute or `~`-prefixed) |
| `git_profile` | Git identity to apply |
| `readonly` | If `true`, `repos pull` skips this repo |
