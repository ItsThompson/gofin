import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { ReactElement } from "react";

const { init, hydrateRoot, workerStart } = vi.hoisted(() => ({
  init: vi.fn(),
  hydrateRoot: vi.fn(),
  workerStart: vi.fn(() => Promise.resolve()),
}));

vi.mock("@sentry/react-router", () => ({
  init,
  // sentry.options.mjs takes this at import time.
  dedupeIntegration: () => ({ name: "Dedupe" }),
}));
vi.mock("react-dom/client", () => ({ hydrateRoot }));
vi.mock("../../mocks/browser", () => ({ worker: { start: workerStart } }));

/** Runs boot() by importing the entry module fresh. */
async function bootEntry(): Promise<void> {
  vi.resetModules();
  await import("../entry.client");
  // boot() is called at module scope and is not awaited by the import, so its
  // dev-mock branch is still pending here.
  await new Promise((resolve) => setTimeout(resolve, 0));
}

describe("entry.client boot", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  describe("with a DSN", () => {
    beforeEach(() => {
      vi.stubEnv("VITE_SENTRY_DSN", "https://publickey@o1.ingest.us.sentry.io/2");
      vi.stubEnv("VITE_SENTRY_RELEASE", "gofin-web@0123456789abcdef");
    });

    it("initializes once with the injected dsn and release", async () => {
      await bootEntry();

      expect(init).toHaveBeenCalledTimes(1);
      const options = init.mock.calls[0][0];
      expect(options.dsn).toBe("https://publickey@o1.ingest.us.sentry.io/2");
      expect(options.release).toBe("gofin-web@0123456789abcdef");
    });

    it("initializes before hydrateRoot, so the first render is observed", async () => {
      await bootEntry();

      expect(hydrateRoot).toHaveBeenCalledTimes(1);
      expect(init.mock.invocationCallOrder[0]).toBeLessThan(
        hydrateRoot.mock.invocationCallOrder[0],
      );
    });

    it("injects a working isNetworkError into the beforeSend chain", async () => {
      await bootEntry();

      const { beforeSend } = init.mock.calls[0][0];
      expect(
        beforeSend(
          { type: undefined, tags: {} },
          { originalException: new TypeError("Failed to fetch") },
        ),
      ).toBeNull();
    });

    it("renders HydratedRouter with no props, so nothing double-reports", async () => {
      // Sentry's setup page renders <HydratedRouter onError={sentryOnError} />,
      // which captures the same route and loader errors that root.tsx's
      // boundary reports.
      await bootEntry();

      const element = hydrateRoot.mock.calls[0][1] as ReactElement;
      expect(element.props).toEqual({});
    });
  });

  describe("without a DSN", () => {
    it("initializes nothing and logs nothing when the value is absent", async () => {
      const consoleCalls: unknown[] = [];
      for (const method of ["log", "info", "warn", "error", "debug"] as const) {
        vi.spyOn(console, method).mockImplementation((...args: unknown[]) => {
          consoleCalls.push(args);
        });
      }

      await bootEntry();

      expect(init).not.toHaveBeenCalled();
      expect(consoleCalls).toEqual([]);
      expect(hydrateRoot).toHaveBeenCalledTimes(1);
    });

    it("initializes nothing when the value is empty", async () => {
      vi.stubEnv("VITE_SENTRY_DSN", "");

      await bootEntry();

      expect(init).not.toHaveBeenCalled();
      expect(hydrateRoot).toHaveBeenCalledTimes(1);
    });
  });

  describe("with the dev mock worker", () => {
    it("still starts the worker before hydrating", async () => {
      // The init sits inside boot() rather than at module scope precisely so this
      // ordering survives: the worker must intercept before the first render.
      vi.spyOn(console, "log").mockImplementation(() => {});
      vi.stubEnv("VITE_SENTRY_DSN", "https://publickey@o1.ingest.us.sentry.io/2");
      vi.stubEnv("VITE_MOCK_API", "true");

      await bootEntry();

      expect(workerStart).toHaveBeenCalledTimes(1);
      expect(init.mock.invocationCallOrder[0]).toBeLessThan(
        workerStart.mock.invocationCallOrder[0],
      );
      expect(workerStart.mock.invocationCallOrder[0]).toBeLessThan(
        hydrateRoot.mock.invocationCallOrder[0],
      );
    });
  });
});
