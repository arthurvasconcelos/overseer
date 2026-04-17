"""plugin — entry-point helpers for daily and status hook plugins."""

import json
import sys
from dataclasses import dataclass
from typing import Callable

from overseer_sdk.context import PluginContext


@dataclass
class StatusResult:
    name: str
    ok: bool
    message: str


def run_main(
    daily_fn: Callable[[PluginContext], str] | None = None,
    status_fn: Callable[[PluginContext], list[StatusResult]] | None = None,
) -> None:
    """Wire up a plugin's daily and/or status hooks from CLI args.

    Call this at the bottom of your plugin script:

        if __name__ == "__main__":
            run_main(daily_fn=my_daily, status_fn=my_status)

    - ``daily_fn`` is called when ``sys.argv[1] == "daily"``.
      It receives the PluginContext and must return a formatted string to print.
    - ``status_fn`` is called when ``sys.argv[1] == "status"``.
      It receives the PluginContext and must return a list of StatusResult objects.
      The results are serialised to JSON on stdout.
    """
    ctx = PluginContext.from_env()
    cmd = sys.argv[1] if len(sys.argv) > 1 else ""

    if cmd == "daily":
        if daily_fn is None:
            sys.exit(0)
        output = daily_fn(ctx)
        sys.stdout.write(output)
        if output and not output.endswith("\n"):
            sys.stdout.write("\n")
    elif cmd == "status":
        if status_fn is None:
            print("[]")
            sys.exit(0)
        results = status_fn(ctx)
        payload = [{"name": r.name, "ok": r.ok, "message": r.message} for r in results]
        print(json.dumps(payload))
    else:
        print(f"unknown command: {cmd!r}", file=sys.stderr)
        sys.exit(1)
