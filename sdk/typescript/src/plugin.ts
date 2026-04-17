/**
 * Entry-point helpers for daily and status hook plugins.
 */

import { loadContext, PluginContext } from "./context.js";

export interface StatusResult {
  name: string;
  ok: boolean;
  message: string;
}

export interface PluginHooks {
  /** Called when the plugin is invoked with the `daily` argument. Must return formatted text. */
  daily?: (ctx: PluginContext) => string | Promise<string>;
  /** Called when the plugin is invoked with the `status` argument. Must return StatusResult[]. */
  status?: (ctx: PluginContext) => StatusResult[] | Promise<StatusResult[]>;
}

/**
 * Wire up a plugin's daily and/or status hooks from CLI args.
 *
 * Call this at the bottom of your plugin script:
 *
 * ```ts
 * runMain({ daily: myDaily, status: myStatus });
 * ```
 *
 * - `daily` is called when `process.argv[2] === "daily"`.
 *   It receives the PluginContext and must return a formatted string to print.
 * - `status` is called when `process.argv[2] === "status"`.
 *   It receives the PluginContext and must return an array of StatusResult objects.
 *   The results are serialised to JSON on stdout.
 */
export async function runMain(hooks: PluginHooks): Promise<void> {
  const ctx = loadContext();
  const cmd = process.argv[2] ?? "";

  if (cmd === "daily") {
    if (!hooks.daily) process.exit(0);
    const output = await hooks.daily(ctx);
    process.stdout.write(output.endsWith("\n") ? output : output + "\n");
  } else if (cmd === "status") {
    if (!hooks.status) {
      console.log("[]");
      process.exit(0);
    }
    const results = await hooks.status(ctx);
    console.log(JSON.stringify(results));
  } else {
    process.stderr.write(`unknown command: ${JSON.stringify(cmd)}\n`);
    process.exit(1);
  }
}
