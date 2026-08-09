import { describe, it, expect, afterEach } from "vitest";
import { isNetworkError } from "@gofin/api";
import {
  clientOptions,
  SHARED_OPTIONS,
  type ClientInitOptions,
} from "../../sentry.options.mjs";

type BeforeSend = ClientInitOptions["beforeSend"];
type SentryErrorEvent = Parameters<BeforeSend>[0];
type SentryEventHint = Parameters<BeforeSend>[1];

function errorEvent(tags: Record<string, string> = {}): SentryErrorEvent {
  return { type: undefined, tags };
}

function hintFor(originalException: unknown): SentryEventHint {
  return { originalException };
}

/** The options every caller in production builds, with the two injected values. */
function options(): ClientInitOptions {
  return clientOptions({
    dsn: "https://publickey@o1.ingest.us.sentry.io/2",
    release: "gofin-web@0123456789abcdef",
    isNetworkError,
  });
}

function setWebdriver(value: boolean): void {
  Object.defineProperty(navigator, "webdriver", {
    value,
    configurable: true,
  });
}

describe("SHARED_OPTIONS", () => {
  it("carries only keys that exist on both platforms' client options", () => {
    // The object is spread into a browser client and a node client, so a
    // browser-only key (the replay rates) or a node-only one
    // (registerEsmLoaderHooks) would be an unknown option on the other.
    expect(Object.keys(SHARED_OPTIONS).sort()).toEqual([
      "dataCollection",
      "environment",
      "ignoreErrors",
      "integrations",
      "tracesSampleRate",
    ]);
  });

  it("locks down every data collection category", () => {
    expect(SHARED_OPTIONS.dataCollection).toEqual({
      userInfo: false,
      cookies: false,
      httpHeaders: { request: false, response: false },
      httpBodies: [],
      urlQueryParams: false,
      graphQL: { document: false, variables: false },
      genAI: { inputs: false, outputs: false },
      databaseQueryData: false,
      stackFrameVariables: false,
    });
  });

  it("sends no traces", () => {
    expect(SHARED_OPTIONS.tracesSampleRate).toBe(0);
  });

  it("ignores the recharts ResizeObserver loop error", () => {
    expect(SHARED_OPTIONS.ignoreErrors).toEqual([
      "ResizeObserver loop completed with undelivered notifications",
    ]);
  });

  it("enables dedupe and nothing else", () => {
    expect(SHARED_OPTIONS.integrations.map((one) => one.name)).toEqual([
      "Dedupe",
    ]);
  });
});

describe("clientOptions", () => {
  it("spreads the shared settings", () => {
    const built = options();

    expect(built.environment).toBe("production");
    expect(built.dataCollection).toEqual(SHARED_OPTIONS.dataCollection);
    expect(built.tracesSampleRate).toBe(0);
  });

  it("sets the three constant tags through initialScope", () => {
    // The JS SDK has no top-level `tags` option, and @sentry/react-router sets
    // `runtime` itself, so a wrong key would leave app and service silently
    // absent from every event while every event-level check still passed.
    expect(options().initialScope.tags).toEqual({
      app: "gofin-web",
      service: "web",
      runtime: "browser",
    });
  });

  it("uses the release string it is given without transforming it", () => {
    expect(options().release).toBe("gofin-web@0123456789abcdef");
    expect(
      clientOptions({
        dsn: "https://publickey@o1.ingest.us.sentry.io/2",
        release: "",
        isNetworkError,
      }).release,
    ).toBe("");
  });

  it("passes the dsn through", () => {
    expect(options().dsn).toBe("https://publickey@o1.ingest.us.sentry.io/2");
  });

  it("disables both Session Replay sample rates", () => {
    expect(options().replaysSessionSampleRate).toBe(0);
    expect(options().replaysOnErrorSampleRate).toBe(0);
  });

  it("denies non-first-party origins and allows first-party ones", () => {
    const { denyUrls } = options();
    const denied = (url: string) => denyUrls.some((rule) => rule.test(url));

    expect(denied("chrome-extension://abcdef/inject.js")).toBe(true);
    expect(denied("moz-extension://abcdef/inject.js")).toBe(true);
    expect(denied("safari-web-extension://abcdef/inject.js")).toBe(true);
    expect(denied("file:///Users/someone/index.html")).toBe(true);
    expect(denied("https://usegofin.com/assets/root-abc123.js")).toBe(false);
  });
});

describe("the client beforeSend chain", () => {
  afterEach(() => {
    setWebdriver(false);
  });

  it("sends an ordinary event", () => {
    const event = errorEvent({ error_kind: "internal" });

    expect(options().beforeSend(event, hintFor(new Error("boom")))).toBe(event);
  });

  it("drops an event tagged expected", () => {
    expect(
      options().beforeSend(
        errorEvent({ expected: "true" }),
        hintFor(new Error("a 422")),
      ),
    ).toBeNull();
  });

  it("compares the expected tag strictly, so a boolean does not match", () => {
    // Tag values are typed Primitive, so a boolean is legal and nothing coerces
    // it. This is why reportError emits the string.
    const event = errorEvent();
    event.tags = { expected: true as unknown as string };

    expect(options().beforeSend(event, hintFor(new Error("a 422")))).toBe(event);
  });

  it("drops a fetch failure classified by the injected isNetworkError", () => {
    expect(
      options().beforeSend(
        errorEvent(),
        hintFor(new TypeError("Failed to fetch")),
      ),
    ).toBeNull();
  });

  it("keeps a TypeError that is not a fetch failure", () => {
    const event = errorEvent();

    expect(
      options().beforeSend(
        event,
        hintFor(new TypeError("Cannot read properties of undefined")),
      ),
    ).toBe(event);
  });

  it("drops everything from an automated browser", () => {
    setWebdriver(true);

    expect(
      options().beforeSend(errorEvent(), hintFor(new Error("boom"))),
    ).toBeNull();
  });
});
