---
title: accounts
weight: 1
---

List all 1Password accounts signed into the `op` CLI.

```bash
overseer accounts
overseer accounts --format json
```

Shows the account URL, email address, and USER UUID. The UUID is what you set as `op_account` in config fields that reference a specific 1Password account.

## JSON output

```json
[
  {
    "url": "my.1password.com",
    "email": "you@example.com",
    "user_uuid": "he7em6mxcdlsewgnzrqldjizi"
  }
]
```
