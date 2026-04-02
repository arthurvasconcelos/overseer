# overseer

Personal machine configuration — dotfiles and the `overseer` CLI.

## Structure

```
overseer/
├── cli/                    ← Go source for the overseer binary
├── dotfiles/
│   ├── shell/
│   │   └── .zshrc
│   └── git/
│       └── .gitconfig
└── scripts/
    └── setup.sh            ← bootstrap script
```

## Bootstrap

Run once on a new machine (or after cloning) to install the binary and wire dotfiles via symlinks:

```bash
bash scripts/setup.sh
```

The script is idempotent — safe to run multiple times. It:

1. Downloads the latest `overseer` binary from GitHub Releases into `~/bin/` (falls back to `go build` if Go is available and the download fails)
2. Symlinks dotfiles into their live locations, backing up any existing files to `~/.overseer-backups/<timestamp>/`

## Symlinks

| overseer path | live location |
|---|---|
| `dotfiles/shell/.zshrc` | `~/.zshrc` |
| `dotfiles/git/.gitconfig` | `~/.gitconfig` |
