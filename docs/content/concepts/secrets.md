---
title: Secrets
weight: 3
---

overseer never stores secret values. Instead, config fields that accept secrets use **1Password URI references** that are resolved at runtime via the `op` CLI.

## Reference format

```
op://vault/item/field
```

Example:

```yaml
integrations:
  github:
    - name: personal
      token: "op://Personal/GitHub PAT/token"
```

When overseer reads this field, it calls `op read op://Personal/GitHub PAT/token` and substitutes the resolved value. The plaintext never touches the config file.

## The `op_account` field

If you have multiple 1Password accounts (e.g. personal and work), specify which account to use per integration:

```yaml
integrations:
  gitlab:
    - name: work
      token: "op://Work/GitLab/token"
      op_account: work
```

`op_account` maps to a 1Password account short name or UUID. Run the following to see your configured accounts and their IDs:

```bash
overseer accounts
```

## `secrets.environments`

Named environments let you inject all secrets from a 1Password account as environment variables for a subprocess:

```yaml
secrets:
  environments:
    p24: "he7em6mxcdlsewgnzrqldjizi"
```

Then use `overseer run --env p24 -- <command>` to run any command with those secrets injected. The alias `p24` maps to the 1Password account ID.

See [`overseer run`](/commands/run) for full usage.
