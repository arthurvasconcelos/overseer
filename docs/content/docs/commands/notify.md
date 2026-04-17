---
title: notify
weight: 13
---

Fire a native OS desktop notification.

```bash
overseer notify "Title" "Body text"
```

Uses the system notification API on macOS (`osascript`) and Linux (`notify-send`). Primarily intended for use inside brain scripts and external plugins that want to alert you when a long-running operation completes.

```bash
# Example: notify when a build finishes
make build && overseer notify "Build done" "Release build succeeded"
```
