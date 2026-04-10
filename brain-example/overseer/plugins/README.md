# Brain plugins

Drop `overseer-*` executables here and overseer will automatically register them
as subcommands. No PATH configuration needed.

## Naming

`overseer-<name>` → `overseer <name>`

## Context

overseer injects an `OVERSEER_CONTEXT` environment variable (JSON) with the
current config path, version, and any resolved secrets declared in the manifest.

See the [plugin SDK](https://github.com/arthurvasconcelos/overseer/tree/main/sdk)
for Python and TypeScript helpers.

## Sidecar manifest (optional)

Place `overseer-<name>.json` alongside the binary to declare metadata:

```json
{
  "description": "My plugin description",
  "secrets": ["github.personal"],
  "hooks": ["daily", "status"]
}
```

| Field | Description |
|---|---|
| `description` | Shown in `overseer --help` and `overseer plugins` |
| `secrets` | Integration references resolved and injected via `OVERSEER_CONTEXT` |
| `hooks` | Participate in `daily` and/or `status` output |

## Hooks

### `daily`

When `hooks` includes `"daily"`, overseer calls `overseer-<name> daily` during
`overseer daily`. Write your section to stdout; it will be printed in the briefing.

### `status`

When `hooks` includes `"status"`, overseer calls `overseer-<name> status` during
`overseer status`. Output must be a JSON array:

```json
[{ "name": "my-check", "ok": true, "message": "all good" }]
```
