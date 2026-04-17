---
title: env
weight: 8
---

Manage environment variable profiles. Profiles map a name to a 1Password secret environment, which injects a set of environment variables into the current shell.

```bash
overseer env list            # list all configured profiles
overseer env show <name>     # print the variables in a profile (values redacted)
eval $(overseer env use <name>)   # apply a profile to the current shell
```

## Config

Profiles are defined in `config.yaml` under `secrets.environments`:

```yaml
secrets:
  environments:
    work: h4e7em6mxcdlsewgnzrqldjizi    # 1Password secret environment ID
    personal: ab1cd2ef3gh4ij5kl6mn7op8
```

The environment ID is the 1Password secret environment identifier. Run `op env list` to find your environment IDs.

## Applying a profile

`overseer env use` prints `export KEY=VALUE` lines to stdout. Use `eval` to apply them in the current shell session:

```bash
eval $(overseer env use work)
```

Variables are resolved from 1Password at the time of the `use` command. They are not stored in any file.
