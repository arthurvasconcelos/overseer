/**
 * Overseer design system for TypeScript plugins.
 *
 * Mirrors the lipgloss palette defined in cli/internal/tui/styles.go.
 * Uses chalk for rendering — add `chalk` to your plugin's dependencies.
 *
 * @example
 *   import { styles, sectionHeader, warnLine } from "@overseer/sdk";
 *
 *   console.log(sectionHeader("GitHub", "3 open PRs"));
 *   console.log(warnLine("auth", "token expired"));
 */

import chalk from "chalk";

// --- Palette --------------------------------------------------------------- //
// 256-colour codes to match the lipgloss palette in styles.go.

export const styles = {
  /** purple — section titles */
  header: (text: string) => chalk.ansi256(99).bold(text),
  /** pink — keys, channels, usernames */
  accent: (text: string) => chalk.ansi256(212)(text),
  /** green — success */
  ok: (text: string) => chalk.ansi256(82).bold(text),
  /** amber — warnings */
  warn: (text: string) => chalk.ansi256(214).bold(text),
  /** red — errors */
  error: (text: string) => chalk.ansi256(196).bold(text),
  /** dark grey — hints, empty state */
  muted: (text: string) => chalk.ansi256(240)(text),
  /** grey — secondary info */
  dim: (text: string) => chalk.ansi256(245)(text),
  /** light — body text */
  normal: (text: string) => chalk.ansi256(252)(text),
};

// --- Helpers --------------------------------------------------------------- //

/**
 * Render a styled section header with an optional badge.
 *
 * Mirrors tui.SectionHeader — output example:
 *   ▸ GitHub  ·  3 open
 */
export function sectionHeader(label: string, badge?: string): string {
  let s = styles.header("▸ " + label);
  if (badge) s += "  " + styles.muted("·  " + badge);
  return s;
}

/**
 * Render a warning line.
 *
 * Mirrors tui.WarnLine — output example:
 *   ⚠  auth: token expired
 */
export function warnLine(label: string, msg: string): string {
  return styles.warn("⚠  " + label + ":") + " " + styles.muted(msg);
}

/**
 * Render a success line.
 *
 * Output example:
 *   ✓  label: message
 */
export function okLine(label: string, msg = ""): string {
  let s = styles.ok("✓  " + label);
  if (msg) s += styles.muted(": " + msg);
  return s;
}

/**
 * Render an error line.
 *
 * Output example:
 *   ✗  label: message
 */
export function errorLine(label: string, msg = ""): string {
  let s = styles.error("✗  " + label);
  if (msg) s += styles.muted(": " + msg);
  return s;
}
