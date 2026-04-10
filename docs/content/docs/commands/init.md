---
title: init
weight: 9
---

Interactive wizard that creates `~/.config/overseer/config.local.yaml` — the machine-local config file.

```bash
overseer init
```

Walks through machine-specific settings: repos path, brain path override, and any values that should not live in the shared brain config. Safe to re-run; existing values are shown as defaults.

`config.local.yaml` is never committed to the brain repo. It holds machine-specific paths and any tokens you prefer to keep out of version control entirely (though using `op://` references in the brain config is the preferred approach for secrets).
