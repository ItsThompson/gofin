import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// vitest's root is this package, and server.js sits at its top level. Read as
// text rather than imported: importing the process entry would start a server.
const source = readFileSync(join(process.cwd(), "server.js"), "utf8");

/** Source lines with comments and blank lines removed. */
const statements = source
  .split("\n")
  .map((line) => line.trim())
  .filter((line) => line.length > 0 && !line.startsWith("//"));

describe("server.js", () => {
  it("imports the Sentry init before anything else", () => {
    // ESM evaluates imports in order, so this placement is what makes the init
    // complete before express loads, before the SSR bundle is imported and
    // before the error middleware can be reached. Nothing at runtime fails
    // loudly if it moves: reports simply become no-ops.
    expect(statements[0]).toBe('import "./instrument.server.mjs";');
    expect(statements[1]).toBe('import express from "express";');
  });

  it("names no workspace package", () => {
    // The file is outside the Vite bundle and @gofin/api ships TypeScript with
    // no build output, so a workspace specifier here does not resolve in the
    // runner image. Bundled helpers arrive through the bundle's own exports.
    expect(source).not.toContain("@gofin");
  });

  it("reads the reporter and the 500 body off the bundle namespace", () => {
    expect(source).toContain("build.reportServerError(error)");
    expect(source).toContain("build.serverErrorBody");
  });

  it("keeps the development branch's ssrFixStacktrace behavior", () => {
    expect(source).toContain("viteDevServer.ssrFixStacktrace(error)");
  });
});
