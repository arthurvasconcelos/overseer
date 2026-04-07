"""Overseer design system for Python plugins.

Mirrors the lipgloss palette defined in cli/internal/tui/styles.go.
Uses rich for rendering — add `rich` to your plugin's dependencies.

Usage:
    from rich.console import Console
    from overseer_sdk.styles import section_header, warn_line, STYLE_OK

    console = Console()
    console.print(section_header("GitHub", "3 open PRs"))
    console.print(warn_line("auth", "token expired"))
"""

from rich.style import Style
from rich.text import Text

# --- Palette --------------------------------------------------------------- #
# 256-colour codes to match the lipgloss palette in styles.go.

STYLE_HEADER = Style(color="color(99)", bold=True)   # purple — section titles
STYLE_ACCENT = Style(color="color(212)")              # pink   — keys, channels, usernames
STYLE_OK     = Style(color="color(82)",  bold=True)  # green  — success
STYLE_WARN   = Style(color="color(214)", bold=True)  # amber  — warnings
STYLE_ERROR  = Style(color="color(196)", bold=True)  # red    — errors
STYLE_MUTED  = Style(color="color(240)")              # dark   — hints, empty state
STYLE_DIM    = Style(color="color(245)")              # grey   — secondary info
STYLE_NORMAL = Style(color="color(252)")              # light  — body text

# --- Helpers --------------------------------------------------------------- #


def section_header(label: str, badge: str = "") -> Text:
    """Render a styled section header with an optional badge.

    Mirrors tui.SectionHeader — output example:
        ▸ GitHub  ·  3 open
    """
    t = Text("▸ " + label, style=STYLE_HEADER)
    if badge:
        t.append("  ·  " + badge, style=STYLE_MUTED)
    return t


def warn_line(label: str, msg: str) -> Text:
    """Render a warning line.

    Mirrors tui.WarnLine — output example:
        ⚠  auth: token expired
    """
    t = Text("⚠  " + label + ":", style=STYLE_WARN)
    t.append(" " + msg, style=STYLE_MUTED)
    return t
