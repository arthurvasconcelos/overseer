import { spawnSync } from "node:child_process";

/**
 * Fire a native desktop notification via `overseer notify`.
 *
 * @param title    - Notification title.
 * @param message  - Notification body.
 * @param subtitle - Optional subtitle (macOS only). Omitted when empty.
 * @throws Error if overseer notify exits non-zero.
 */
export function notify(title: string, message: string, subtitle = ""): void {
  const args = ["notify", title, message];
  if (subtitle) {
    args.push("--subtitle", subtitle);
  }
  const result = spawnSync("overseer", args, { stdio: "inherit" });
  if (result.status !== 0) {
    throw new Error(`overseer notify exited with status ${result.status}`);
  }
}
