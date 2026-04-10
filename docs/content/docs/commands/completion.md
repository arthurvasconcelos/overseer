---
title: completion
weight: 4
---

Generate shell completion scripts for overseer.

## Zsh

```bash
# Source in current session
source <(overseer completion zsh)

# Persist across sessions
echo 'source <(overseer completion zsh)' >> ~/.zshrc
```

## Bash

```bash
echo 'source <(overseer completion bash)' >> ~/.bashrc
```

## Fish

```bash
overseer completion fish > ~/.config/fish/completions/overseer.fish
```

## PowerShell

```bash
overseer completion powershell
```
