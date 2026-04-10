---
title: run
weight: 14
---

Run a command with secrets resolved and injected as environment variables.

```bash
overseer run [flags] -- <cmd> [args...]
```

## Flags

| Flag | Description |
|---|---|
| `--gitlab <name>` | GitLab instance name from config — injects `GITLAB_TOKEN` and `GITLAB_HOST` |
| `--github <name>` | GitHub instance name from config — injects `GITHUB_TOKEN` |
| `--env <name>` | 1Password environment alias or account ID — injects all secrets; mutually exclusive with `--gitlab`/`--github` |

## Examples

```bash
# Authenticated curl against a configured GitLab instance
overseer run --gitlab work -- curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" \
  https://$GITLAB_HOST/api/v4/projects

# gh CLI with a configured GitHub token
overseer run --github personal -- gh repo list

# Run a Makefile target with a 1Password environment's secrets
overseer run --env p24 -- make deploy

# Use a raw 1Password account ID instead of an alias
overseer run --env he7em6mxcdlsewgnzrqldjizi -- make deploy
```

## How it works

- `--gitlab <name>` looks up the matching entry in `integrations.gitlab[]`, resolves its token via `op read`, and exports `GITLAB_TOKEN` and `GITLAB_HOST` before running the command.
- `--github <name>` does the same for `integrations.github[]`, exporting `GITHUB_TOKEN`.
- `--env <name>` resolves the alias through `secrets.environments` to a 1Password account ID, then uses `op run` to inject all secrets from that account.

See [Concepts → Secrets](/docs/concepts/secrets) for how secret resolution works.
