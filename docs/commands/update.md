# update


Check for and apply a new overseer release.

```bash
overseer update
```

Fetches the latest release from GitHub and compares it against the running version. When a newer version exists:

- **Installed with Homebrew:** runs `brew upgrade arthurvasconcelos/tap/overseer`. The `brew` comes from the install location, so this works even when `brew` isn't on your `PATH`. If an earlier `overseer update` left a separate copy in `~/bin/overseer`, you get a warning to remove it.
- **Otherwise:** downloads the release and installs it to `~/bin/overseer`.
