import { describe, it, expect, afterEach } from "vitest";
import { loadContext, getSecret } from "../context.js";

const CTX = {
  version: "1.2.3",
  config_path: "/home/user/.config",
  secrets: { github: { token: "ghp_abc" } },
};

describe("loadContext", () => {
  afterEach(() => {
    delete process.env.OVERSEER_CONTEXT;
  });

  it("parses env var into PluginContext", () => {
    process.env.OVERSEER_CONTEXT = JSON.stringify(CTX);
    const ctx = loadContext();
    expect(ctx.version).toBe("1.2.3");
    expect(ctx.config_path).toBe("/home/user/.config");
    expect(ctx.secrets).toEqual({ github: { token: "ghp_abc" } });
  });

  it("defaults secrets to empty when omitted", () => {
    process.env.OVERSEER_CONTEXT = JSON.stringify({
      version: "1.0.0",
      config_path: "/tmp",
    });
    const ctx = loadContext();
    expect(ctx.secrets).toBeUndefined();
  });

  it("throws when OVERSEER_CONTEXT is not set", () => {
    delete process.env.OVERSEER_CONTEXT;
    expect(() => loadContext()).toThrow("OVERSEER_CONTEXT is not set");
  });
});

describe("getSecret", () => {
  it("returns value for known ref and key", () => {
    const ctx = { version: "1", config_path: "/tmp", secrets: { github: { token: "abc" } } };
    expect(getSecret(ctx, "github", "token")).toBe("abc");
  });

  it("throws for unknown ref", () => {
    const ctx = { version: "1", config_path: "/tmp", secrets: {} };
    expect(() => getSecret(ctx, "gitlab", "token")).toThrow('Secret "gitlab/token" not found');
  });

  it("throws for unknown key", () => {
    const ctx = { version: "1", config_path: "/tmp", secrets: { github: { token: "abc" } } };
    expect(() => getSecret(ctx, "github", "missing")).toThrow('Secret "github/missing" not found');
  });
});
