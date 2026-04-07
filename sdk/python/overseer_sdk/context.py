import json
import os
from dataclasses import dataclass, field


@dataclass
class PluginContext:
    version: str
    config_path: str
    secrets: dict[str, dict[str, str]] = field(default_factory=dict)

    @classmethod
    def from_env(cls) -> "PluginContext":
        raw = os.environ.get("OVERSEER_CONTEXT")
        if not raw:
            raise RuntimeError(
                "OVERSEER_CONTEXT is not set — is this plugin running under overseer?"
            )
        data = json.loads(raw)
        return cls(
            version=data["version"],
            config_path=data["config_path"],
            secrets=data.get("secrets", {}),
        )

    def secret(self, ref: str, key: str) -> str:
        """Return a resolved secret value by integration ref and key.

        Example:
            token = ctx.secret("github.personal", "token")
        """
        try:
            return self.secrets[ref][key]
        except KeyError:
            raise KeyError(
                f"Secret {ref!r}/{key!r} not found — "
                "did you declare it in your plugin manifest?"
            )
