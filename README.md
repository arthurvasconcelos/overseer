# overseer

A personal machine management CLI. Wraps daily workflows — briefings, git identities, repo management, notes, PR reviews, Homebrew, and more — into a single binary.

Personal files (dotfiles, Brewfile, config) live in a separate **brain** directory that you own and version independently. overseer manages that brain; the two repos are decoupled.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/arthurvasconcelos/overseer/main/scripts/install.sh | bash
```

Installs the latest binary to `~/bin/`. Make sure `~/bin` is on your `PATH`.

---

## First-time setup

```bash
overseer setup
```

The interactive wizard walks through everything in one session:
brain path, git remote, machine settings, directory scaffolding, dotfile wiring, and Brew packages.

Safe to re-run anytime — existing values are shown as defaults and nothing is overwritten without your input.

---

## The brain

Your brain is a directory (typically a private git repo) that overseer manages as the single source of truth for your personal config. Use `overseer brain` commands to create and apply it.

```
brain/
  overseer/
    config.yaml          # integrations, git profiles, repos, etc.
    dotfiles/            # mirrors ~/ — symlinked by overseer brain setup
      .zshrc             →  ~/.zshrc
      .gitconfig         →  ~/.gitconfig
      .config/
        starship.toml    →  ~/.config/starship.toml
    Brewfile             # your Homebrew packages
    Brewfile.local       # machine-specific packages (gitignored)
    plugins/             # personal overseer-* plugins
```

See [brain-example/](brain-example/) for a fully commented example layout.

### Brain path resolution

overseer finds your brain in this order:

1. `OVERSEER_BRAIN` env var
2. `system.brain_path` in `config.local.yaml`
3. `~/brain` (default)

### Referenced paths

Not everything overseer uses lives in the brain. Some directories are **referenced by path** in `config.yaml` and stay wherever they already are:

- **Obsidian vaults** — configure `obsidian.vault_path` with an absolute path
- **Repos workspace** — configure `system.overseer_home` (where managed repos are cloned)

---

## Commands

| Command | Description |
|---|---|
| `overseer setup` | Interactive wizard: brain, machine, dotfiles, packages |
| `overseer brain setup` | Re-apply dotfiles and Brew packages without the wizard |
| `overseer brain status` | Show brain directory health |
| `overseer brain path` | Print the resolved brain path |
| `overseer daily` | Morning briefing (Jira, Slack, Calendar) |
| `overseer prs` | Open PRs across GitHub and GitLab |
| `overseer note` | Create and search notes in Obsidian |
| `overseer status` | Health check all integrations |
| `overseer repos` | Manage and sync git repos |
| `overseer git` | Apply git identity profiles |
| `overseer brew` | Manage Homebrew packages via Brewfile |
| `overseer config` | Show active config and brain path |
| `overseer update` | Self-update the binary |
| `overseer run` | Run a command with secrets injected |

---

## Config reference

The brain's `overseer/config.yaml` is the portable config. `config.local.yaml` (at `~/.config/overseer/config.local.yaml`) holds machine-specific overrides and is never committed.

See [brain-example/overseer/config.yaml.example](brain-example/overseer/config.yaml.example) for a fully commented reference of all keys.

---

## Plugins

Drop `overseer-<name>` executables in `brain/overseer/plugins/` (or anywhere on `PATH`) and they are automatically registered as `overseer <name>` subcommands.

See [brain-example/overseer/plugins/README.md](brain-example/overseer/plugins/README.md) and the [sdk/](sdk/) directory for Python and TypeScript helpers.

---

## Build from source

Requires Go 1.21+.

```bash
bash scripts/setup.sh
```

Or with make:

```bash
make dev
```

Builds to `~/bin/overseer`.

### Local development

To use your brain during local development without setting `OVERSEER_BRAIN`:

```bash
bash scripts/dev.sh
```

Creates `repos/brain → ~/brain` symlink (gitignored).
