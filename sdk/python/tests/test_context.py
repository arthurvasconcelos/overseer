import json

import pytest

from overseer_sdk.context import PluginContext

CTX = {
    "version": "1.2.3",
    "config_path": "/home/user/.config",
    "secrets": {"github": {"token": "ghp_abc"}},
}


def test_from_env_happy_path(monkeypatch):
    monkeypatch.setenv("OVERSEER_CONTEXT", json.dumps(CTX))
    ctx = PluginContext.from_env()
    assert ctx.version == "1.2.3"
    assert ctx.config_path == "/home/user/.config"
    assert ctx.secrets == {"github": {"token": "ghp_abc"}}


def test_from_env_no_secrets(monkeypatch):
    monkeypatch.setenv(
        "OVERSEER_CONTEXT",
        json.dumps({"version": "1.0.0", "config_path": "/tmp"}),
    )
    ctx = PluginContext.from_env()
    assert ctx.secrets == {}


def test_from_env_missing_env(monkeypatch):
    monkeypatch.delenv("OVERSEER_CONTEXT", raising=False)
    with pytest.raises(RuntimeError, match="OVERSEER_CONTEXT is not set"):
        PluginContext.from_env()


def test_secret_happy_path(monkeypatch):
    monkeypatch.setenv("OVERSEER_CONTEXT", json.dumps(CTX))
    ctx = PluginContext.from_env()
    assert ctx.secret("github", "token") == "ghp_abc"


def test_secret_missing_ref(monkeypatch):
    monkeypatch.setenv("OVERSEER_CONTEXT", json.dumps(CTX))
    ctx = PluginContext.from_env()
    with pytest.raises(KeyError):
        ctx.secret("gitlab", "token")


def test_secret_missing_key(monkeypatch):
    monkeypatch.setenv("OVERSEER_CONTEXT", json.dumps(CTX))
    ctx = PluginContext.from_env()
    with pytest.raises(KeyError):
        ctx.secret("github", "nonexistent")
