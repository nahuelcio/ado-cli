import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("package metadata", () => {
  const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf-8"));
  const cliSource = readFileSync(new URL("../src/bin/opencode-ado.ts", import.meta.url), "utf-8");
  const tuiSource = readFileSync(new URL("../src/tui.tsx", import.meta.url), "utf-8");

  it("publishes the TUI source expected by OpenCode", () => {
    expect(pkg.exports["./tui"].default).toBe("./dist/tui.tsx");
    expect(pkg.files).toContain("dist");
  });

  it("build copies the TUI TSX file into dist", () => {
    expect(pkg.scripts.build).toContain("scripts/prepare-tui-dist.mjs");
  });

  it("init registers both server and TUI plugin configs", () => {
    expect(cliSource).toContain("syncTuiPluginConfig");
    expect(cliSource).toContain("tui.json");
    expect(cliSource).toContain("TUI sidebar plugin added to tui.json");
  });

  it("provides a non-interactive sync command for existing profiles", () => {
    expect(cliSource).toContain("function syncExistingConfig");
    expect(cliSource).toContain("npx @nahuelcio/opencode-ado sync");
    expect(cliSource).toContain('command === "sync"');
  });

  it("pins OpenCode config to the running package version to avoid stale latest cache", () => {
    expect(cliSource).toContain("getVersionedPluginSpec");
    expect(cliSource).toContain("pkg.version");
    expect(cliSource).toContain("startsWith(`${PLUGIN_SPEC}@`)");
  });

  it("renders sidebar state reactively after PR loading completes", () => {
    expect(tuiSource).toContain("Switch");
    expect(tuiSource).toContain("Match");
    expect(tuiSource).not.toContain('if (d().status === "loading") return');
  });
});
