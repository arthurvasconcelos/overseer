---
title: Installation
weight: 1
---

## Homebrew (macOS)

```bash
brew install arthurvasconcelos/tap/overseer
```

## Manual install

Downloads and installs the latest binary to `~/bin/`:

```bash
curl -fsSL https://raw.githubusercontent.com/arthurvasconcelos/overseer/main/scripts/install.sh | bash
```

Make sure `~/bin` is on your `PATH`:

```bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
```

## Build from source

Requires Go 1.21+.

```bash
git clone https://github.com/arthurvasconcelos/overseer.git
cd overseer
make dev
```

Builds and installs to `~/bin/overseer`.

---

## First-time setup

Run the interactive wizard after installing:

```bash
overseer setup
```

This walks through brain path, git remote, machine settings, directory scaffolding, dotfile wiring, and Brew packages in one session. Existing values are shown as defaults and nothing is overwritten without your input. Safe to re-run at any time.

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
