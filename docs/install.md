---
description: Install overseer via Homebrew on macOS, Linux or WSL, or from source, run the setup wizard, and configure shell completions.
---

# Installation


## Homebrew (macOS, Linux, WSL)

overseer ships as a Homebrew cask. The same command works on macOS, Linux and WSL.

**Stable channel** (recommended):

```bash
brew install arthurvasconcelos/tap/overseer
```

**Beta channel** (pre-release builds):

```bash
brew install arthurvasconcelos/tap/overseer-beta
```

Upgrade with:

```bash
brew update && brew upgrade arthurvasconcelos/tap/overseer
```

On Linux and WSL, casks need a recent Homebrew. Run `brew update` first if the install complains about casks.

::: tip On WSL?
1Password and SSH need a few extra steps under WSL. See [WSL](/concepts/wsl) before running `overseer setup`.
:::

### Troubleshooting

**`undefined method 'cask' for module Formulary::FormulaNamespace…`**: your local copy of the tap is from before the cask moved to the `Casks/` directory (fixed in v0.27.1). Refresh it:

```bash
brew update
brew install arthurvasconcelos/tap/overseer
```

If it still fails, re-tap: `brew untap arthurvasconcelos/tap && brew install arthurvasconcelos/tap/overseer`.

## Manual install

Downloads and installs the latest binary to `~/bin/`:

**Stable channel** (recommended):

```bash
curl -fsSL https://raw.githubusercontent.com/arthurvasconcelos/overseer/main/scripts/install.sh | bash
```

**Beta channel** (pre-release builds):

```bash
curl -fsSL https://raw.githubusercontent.com/arthurvasconcelos/overseer/main/scripts/install.sh | bash -s -- --channel beta
```

Make sure `~/bin` is on your `PATH`:

```bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
```

## Build from source

Requires Go (the version in `cli/go.mod`) and [just](https://github.com/casey/just).

```bash
git clone https://github.com/arthurvasconcelos/overseer.git
cd overseer
just dev
```

Builds and installs to `~/bin/overseer`.

---

## First-time setup

Run the interactive wizard after installing:

```bash
overseer setup
```

This walks through brain path, git remote, machine settings, your first git profile, directory scaffolding, dotfile wiring, and Brew packages in one session. Existing values are shown as defaults and nothing is overwritten without your input. Safe to re-run at any time. See [`overseer setup`](/commands/setup) for each step.

If you already have a brain and only want to re-apply config without the full wizard:

```bash
overseer brain setup
```

---

## Shell completions

```bash
# Zsh
echo 'source <(overseer completion zsh)' >> ~/.zshrc

# Bash
echo 'source <(overseer completion bash)' >> ~/.bashrc

# Fish
overseer completion fish > ~/.config/fish/completions/overseer.fish
```
