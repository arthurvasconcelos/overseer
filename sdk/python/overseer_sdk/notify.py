"""notify — shell out to `overseer notify` to fire a native desktop notification."""

import subprocess


def notify(title: str, message: str, subtitle: str = "") -> None:
    """Fire a native desktop notification via overseer notify.

    Args:
        title:    Notification title.
        message:  Notification body.
        subtitle: Optional subtitle (macOS only). Omitted when empty.

    Raises:
        subprocess.CalledProcessError: if overseer notify exits non-zero.
    """
    cmd = ["overseer", "notify", title, message]
    if subtitle:
        cmd += ["--subtitle", subtitle]
    subprocess.run(cmd, check=True)
