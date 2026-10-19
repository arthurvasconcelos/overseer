# init


Interactive wizard that creates `config.local.yaml`, the machine-local config file (`~/.config/overseer/` on Linux and WSL, `~/Library/Application Support/overseer/` on macOS).

```bash
overseer init
```

Walks through machine-specific settings: brain path, repos path, and the SSH signing program (`gpg_ssh_program`, which defaults to 1Password's signer for the platform, including `op-ssh-sign-wsl.exe` under [WSL](/concepts/wsl)). Asks before overwriting an existing file. `overseer setup` covers the same settings as part of the full wizard.

`config.local.yaml` is never committed to the brain repo. It holds machine-specific paths and any tokens you prefer to keep out of version control entirely (though using `op://` references in the brain config is the preferred approach for secrets).
