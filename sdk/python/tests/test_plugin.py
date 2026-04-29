import json
import sys

import pytest

from overseer_sdk.plugin import StatusResult, run_main

CTX = json.dumps({"version": "1.0.0", "config_path": "/tmp/config.yaml", "secrets": {}})


def test_daily_routing(monkeypatch, capsys):
    monkeypatch.setenv("OVERSEER_CONTEXT", CTX)
    monkeypatch.setattr(sys, "argv", ["plugin", "daily"])
    run_main(daily_fn=lambda ctx: "hello")
    assert capsys.readouterr().out == "hello\n"


def test_daily_appends_newline(monkeypatch, capsys):
    monkeypatch.setenv("OVERSEER_CONTEXT", CTX)
    monkeypatch.setattr(sys, "argv", ["plugin", "daily"])
    run_main(daily_fn=lambda ctx: "no newline")
    assert capsys.readouterr().out == "no newline\n"


def test_daily_no_double_newline(monkeypatch, capsys):
    monkeypatch.setenv("OVERSEER_CONTEXT", CTX)
    monkeypatch.setattr(sys, "argv", ["plugin", "daily"])
    run_main(daily_fn=lambda ctx: "already\n")
    assert capsys.readouterr().out == "already\n"


def test_status_routing(monkeypatch, capsys):
    monkeypatch.setenv("OVERSEER_CONTEXT", CTX)
    monkeypatch.setattr(sys, "argv", ["plugin", "status"])
    results = [StatusResult(name="github", ok=True, message="all good")]
    run_main(status_fn=lambda ctx: results)
    out = capsys.readouterr().out
    assert json.loads(out) == [{"name": "github", "ok": True, "message": "all good"}]


def test_status_multiple_results(monkeypatch, capsys):
    monkeypatch.setenv("OVERSEER_CONTEXT", CTX)
    monkeypatch.setattr(sys, "argv", ["plugin", "status"])
    results = [
        StatusResult(name="github", ok=True, message="ok"),
        StatusResult(name="jira", ok=False, message="unreachable"),
    ]
    run_main(status_fn=lambda ctx: results)
    out = json.loads(capsys.readouterr().out)
    assert len(out) == 2
    assert out[1]["name"] == "jira"
    assert out[1]["ok"] is False


def test_status_no_hook_exits_zero(monkeypatch, capsys):
    monkeypatch.setenv("OVERSEER_CONTEXT", CTX)
    monkeypatch.setattr(sys, "argv", ["plugin", "status"])
    with pytest.raises(SystemExit) as exc:
        run_main()
    assert exc.value.code == 0
    assert capsys.readouterr().out.strip() == "[]"


def test_daily_no_hook_exits_zero(monkeypatch):
    monkeypatch.setenv("OVERSEER_CONTEXT", CTX)
    monkeypatch.setattr(sys, "argv", ["plugin", "daily"])
    with pytest.raises(SystemExit) as exc:
        run_main()
    assert exc.value.code == 0


def test_unknown_command_exits_one(monkeypatch):
    monkeypatch.setenv("OVERSEER_CONTEXT", CTX)
    monkeypatch.setattr(sys, "argv", ["plugin", "bogus"])
    with pytest.raises(SystemExit) as exc:
        run_main()
    assert exc.value.code == 1
