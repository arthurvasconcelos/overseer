---
title: repos
weight: 13
---

Manage git repositories defined in the `repos[]` section of config.

## Subcommands

### `overseer repos status`

Show git status across all managed repos: branch, clean/dirty, and list of changed files.

```bash
overseer repos status
overseer repos status --format json
```

JSON output:

```json
[
  {
    "name": "brain",
    "path": "/Users/you/brain",
    "readonly": false,
    "cloned": true,
    "branch": "main",
    "clean": true,
    "changes": []
  },
  {
    "name": "work-api",
    "path": "/Users/you/repos/work/api",
    "readonly": false,
    "cloned": true,
    "branch": "feature/auth",
    "clean": false,
    "changes": ["M src/auth.go", "?? src/auth_test.go"]
  }
]
```

### `overseer repos pull`

Pull (or clone if missing) all managed repos. Repos with `readonly: true` are skipped.

```bash
overseer repos pull
```

### `overseer repos setup`

Apply the configured git profile to each already-cloned repo. Useful after adding a new profile or changing which profile a repo should use.

```bash
overseer repos setup
```

## Config

Repos are defined in `repos[]` in config. See [Concepts → Config](/docs/concepts/config#repos) for all fields.
