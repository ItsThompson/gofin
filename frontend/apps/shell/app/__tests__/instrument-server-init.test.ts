import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const { init } = vi.hoisted(() => ({ init: vi.fn() }));

vi.mock("@sentry/react-router", () => ({
  init,
  // sentry.options.mjs takes this at import time.
  dedupeIntegration: () => ({ name: "Dedupe" }),
}));

/** Runs the init module fresh, since it initializes at module scope. */
async function importInstrument(): Promise<void> {
  vi.resetModules();
  await import("../../instrument.server.mjs");
}

describe("instrument.server", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  describe("with a DSN", () => {
    beforeEach(() => {
      vi.stubEnv("SENTRY_DSN", "https://publickey@o1.ingest.us.sentry.io/3");
      vi.stubEnv("SENTRY_RELEASE", "0123456789abcdef");
    });

    it("initializes once with the server options", async () => {
      await importInstrument();

      expect(init).toHaveBeenCalledTimes(1);
      const options = init.mock.calls[0][0];
      expect(options.dsn).toBe("https://publickey@o1.ingest.us.sentry.io/3");
      expect(options.initialScope.tags).toEqual({
        app: "gofin-web",
        service: "web",
        runtime: "node",
      });
      expect(options.registerEsmLoaderHooks).toBe(false);
      expect(options.dataCollection.userInfo).toBe(false);
    });

    it("prefixes the bare SHA exactly once", async () => {
      await importInstrument();

      expect(init.mock.calls[0][0].release).toBe("gofin-web@0123456789abcdef");
    });

    it("sends no release when the SHA is empty", async () => {
      // gofin-web@ alone is a release name that matches no uploaded source map,
      // which is worse than no release at all.
      vi.stubEnv("SENTRY_RELEASE", "");
      await importInstrument();

      expect(init.mock.calls[0][0].release).toBeUndefined();
    });
  });

  describe("without a DSN", () => {
    it("does not initialize when the value is empty", async () => {
      vi.stubEnv("SENTRY_DSN", "");
      vi.stubEnv("SENTRY_RELEASE", "0123456789abcdef");
      await importInstrument();

      expect(init).not.toHaveBeenCalled();
    });

    it("does not initialize when the value is absent", async () => {
      vi.stubEnv("SENTRY_DSN", undefined);
      await importInstrument();

      expect(init).not.toHaveBeenCalled();
    });
  });
});
