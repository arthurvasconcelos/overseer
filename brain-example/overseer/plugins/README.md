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
  "secrets": ["github.personal"]
}
```
