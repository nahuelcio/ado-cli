import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("package metadata", () => {
  const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf-8"));

  it("publishes the TUI source expected by OpenCode", () => {
    expect(pkg.exports["./tui"].default).toBe("./dist/tui.tsx");
    expect(pkg.files).toContain("dist");
  });

  it("build copies the TUI TSX file into dist", () => {
    expect(pkg.scripts.build).toContain("scripts/prepare-tui-dist.mjs");
  });
});
