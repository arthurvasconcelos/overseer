---
title: config
weight: 5
---

Show the active merged configuration and related utilities.

## Usage

```bash
overseer config                   # human-readable summary
overseer config --format json     # full merged config as JSON
overseer config schema            # print the JSON Schema for config.yaml
```

The displayed config is the result of merging `brain/overseer/config.yaml` with `~/.config/overseer/config.local.yaml`. Local values take precedence.

Secret values (fields containing `op://` references) are shown as their reference strings, never as resolved plaintext.

## JSON Schema

`overseer config schema` prints the full JSON Schema for `config.yaml`. You can use this to enable inline validation and autocomplete in editors that support YAML Language Server:

```yaml
# brain/overseer/config.yaml
# yaml-language-server: $schema=https://arthurvasconcelos.github.io/overseer/schema.json
```

See [Concepts → Config](/concepts/config) for all config keys.
