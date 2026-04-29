import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from "vitest";
import { runMain } from "../plugin.js";
import type { StatusResult } from "../plugin.js";

const CTX_JSON = JSON.stringify({
  version: "1.0.0",
  config_path: "/tmp/config.yaml",
  secrets: {},
});

class ExitError extends Error {
  constructor(public code: number) {
    super(`process.exit(${code})`);
  }
}

describe("runMain", () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let stdoutSpy: MockInstance<any>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let consoleSpy: MockInstance<any>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let stderrSpy: MockInstance<any>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let exitSpy: MockInstance<any>;

  beforeEach(() => {
    process.env.OVERSEER_CONTEXT = CTX_JSON;
    stdoutSpy = vi.spyOn(process.stdout, "write").mockImplementation(() => true);
    consoleSpy = vi.spyOn(console, "log").mockImplementation(() => {});
    stderrSpy = vi.spyOn(process.stderr, "write").mockImplementation(() => true);
    exitSpy = vi.spyOn(process, "exit").mockImplementation((code) => {
      throw new ExitError(Number(code ?? 0));
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    delete process.env.OVERSEER_CONTEXT;
  });

  it("routes daily and writes output", async () => {
    process.argv[2] = "daily";
    await runMain({ daily: async () => "hello" });
    expect(stdoutSpy).toHaveBeenCalledWith("hello\n");
  });

  it("appends newline when missing", async () => {
    process.argv[2] = "daily";
    await runMain({ daily: async () => "no newline" });
    expect(stdoutSpy).toHaveBeenCalledWith("no newline\n");
  });

  it("does not double newline", async () => {
    process.argv[2] = "daily";
    await runMain({ daily: async () => "already\n" });
    expect(stdoutSpy).toHaveBeenCalledWith("already\n");
  });

  it("routes status and prints JSON", async () => {
    process.argv[2] = "status";
    const results: StatusResult[] = [{ name: "github", ok: true, message: "all good" }];
    await runMain({ status: async () => results });
    expect(consoleSpy).toHaveBeenCalledWith(JSON.stringify(results));
  });

  it("serialises multiple status results correctly", async () => {
    process.argv[2] = "status";
    const results: StatusResult[] = [
      { name: "github", ok: true, message: "ok" },
      { name: "jira", ok: false, message: "unreachable" },
    ];
    await runMain({ status: async () => results });
    const printed = JSON.parse(consoleSpy.mock.calls[0][0] as string);
    expect(printed).toHaveLength(2);
    expect(printed[1]).toMatchObject({ name: "jira", ok: false });
  });

  it("prints empty array and exits 0 when status hook absent", async () => {
    process.argv[2] = "status";
    await expect(runMain({})).rejects.toThrow(ExitError);
    expect(consoleSpy).toHaveBeenCalledWith("[]");
    expect(exitSpy).toHaveBeenCalledWith(0);
  });

  it("exits 0 when daily hook absent", async () => {
    process.argv[2] = "daily";
    await expect(runMain({})).rejects.toThrow(ExitError);
    expect(exitSpy).toHaveBeenCalledWith(0);
  });

  it("exits 1 on unknown command", async () => {
    process.argv[2] = "bogus";
    await expect(runMain({})).rejects.toThrow(ExitError);
    expect(exitSpy).toHaveBeenCalledWith(1);
  });
});
