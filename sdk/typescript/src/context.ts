export interface PluginSecrets {
  [ref: string]: {
    [key: string]: string;
  };
}

export interface PluginContext {
  version: string;
  config_path: string;
  secrets: PluginSecrets;
}

/**
 * Load the Overseer plugin context from the OVERSEER_CONTEXT environment variable.
 * Throws if the variable is not set (i.e. the binary is not running under overseer).
 */
export function loadContext(): PluginContext {
  const raw = process.env.OVERSEER_CONTEXT;
  if (!raw) {
    throw new Error(
      "OVERSEER_CONTEXT is not set — is this plugin running under overseer?"
    );
  }
  return JSON.parse(raw) as PluginContext;
}

/**
 * Return a resolved secret value by integration ref and key.
 *
 * @example
 *   const token = getSecret(ctx, "github.personal", "token");
 */
export function getSecret(ctx: PluginContext, ref: string, key: string): string {
  const value = ctx.secrets?.[ref]?.[key];
  if (value === undefined) {
    throw new Error(
      `Secret "${ref}/${key}" not found — did you declare it in your plugin manifest?`
    );
  }
  return value;
}
