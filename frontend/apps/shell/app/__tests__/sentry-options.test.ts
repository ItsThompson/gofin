import { describe, it, expect, afterEach } from "vitest";
import { eventFiltersIntegration } from "@sentry/react-router";
import { isNetworkError } from "@gofin/api";
import {
  clientOptions,
  serverOptions,
  SHARED_OPTIONS,
  type ClientInitOptions,
  type ServerInitOptions,
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

/** The options instrument.server.mjs builds, with the prefix already applied. */
function serverBuilt(): ServerInitOptions {
  return serverOptions({
    dsn: "https://publickey@o1.ingest.us.sentry.io/3",
    release: "gofin-web@0123456789abcdef",
  });
}

/**
 * The three messages `throwIfPotentialCSRFAttack` throws, read from
 * `react-router@7.15.0`'s production bundle. Every one ends in the clause the
 * `ignoreErrors` entry matches.
 */
const CSRF_REJECTION_MESSAGES = [
  "`origin` header is not a valid URL. Aborting the action.",
  "host header does not match `origin` header from a forwarded action request. Aborting the action.",
  "`x-forwarded-host` or `host` headers are not provided. One of these is needed to compare the `origin` header from a forwarded action request. Aborting the action.",
];

type ProcessEvent = NonNullable<
  ReturnType<typeof eventFiltersIntegration>["processEvent"]
>;

/**
 * Runs an exception message through the SDK's real event filter with the real
 * shared options, rather than reimplementing its matching semantics here. The
 * filter reads nothing off the client but its options.
 */
function filterEvent(value: string) {
  const integration = eventFiltersIntegration();
  if (!integration.processEvent) {
    throw new Error("the SDK's event filter no longer exposes processEvent");
  }

  const event: Parameters<ProcessEvent>[0] = {
    exception: { values: [{ type: "Error", value }] },
  };
  const client = {
    getOptions: () => SHARED_OPTIONS,
  } as unknown as Parameters<ProcessEvent>[2];

  return integration.processEvent(event, {}, client);
}

function setWebdriver(value: boolean): void {
  Object.defineProperty(navigator, "webdriver", {
    value,
    configurable: true,
  });
}

/** Spec 06's lockdown, spelled out so a permissive-but-identical set fails. */
const LOCKED_DATA_COLLECTION = {
  userInfo: false,
  cookies: false,
  httpHeaders: { request: false, response: false },
  httpBodies: [],
  urlQueryParams: false,
  graphQL: { document: false, variables: false },
  genAI: { inputs: false, outputs: false },
  databaseQueryData: false,
  stackFrameVariables: false,
};

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
    expect(SHARED_OPTIONS.dataCollection).toEqual(LOCKED_DATA_COLLECTION);
  });

  it("sends no traces", () => {
    expect(SHARED_OPTIONS.tracesSampleRate).toBe(0);
  });

  it("ignores the recharts ResizeObserver loop error and the router's CSRF rejections", () => {
    expect(SHARED_OPTIONS.ignoreErrors).toEqual([
      "ResizeObserver loop completed with undelivered notifications",
      "Aborting the action.",
    ]);
  });

  it("drops all three CSRF rejection messages through the SDK's own filter", () => {
    // Every cross-origin POST document request reaches
    // throwIfPotentialCSRFAttack, which throws a plain Error, so neither
    // handleError's sub-500 rule nor beforeSend can see it. Third parties can
    // trigger it and no in-app path can. One substring covers all three
    // messages react-router 7.15.0 throws; a narrower entry would leave two
    // remotely triggerable events billing against the quota.
    for (const message of CSRF_REJECTION_MESSAGES) {
      expect(filterEvent(message)).toBeNull();
    }
  });

  it("keeps an ordinary application error", () => {
    expect(filterEvent("Cannot read properties of undefined")).not.toBeNull();
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

describe("serverOptions", () => {
  it("spreads the shared settings", () => {
    const built = serverBuilt();

    expect(built.environment).toBe("production");
    expect(built.tracesSampleRate).toBe(0);
    expect(built.ignoreErrors).toEqual(SHARED_OPTIONS.ignoreErrors);
  });

  it("sets the three constant tags through initialScope, with runtime node", () => {
    // runtime alone would pass even with the option key wrong, because
    // @sentry/react-router's server init calls setTag("runtime", "node") itself.
    expect(serverBuilt().initialScope.tags).toEqual({
      app: "gofin-web",
      service: "web",
      runtime: "node",
    });
  });

  it("disables the ESM loader hooks", () => {
    expect(serverBuilt().registerEsmLoaderHooks).toBe(false);
  });

  it("uses the release string it is given without transforming it", () => {
    // The caller applies the gofin-web@ prefix, so a builder that prefixed too
    // would emit gofin-web@gofin-web@<sha> and split one deploy in two.
    expect(serverBuilt().release).toBe("gofin-web@0123456789abcdef");
    expect(
      serverOptions({ dsn: "https://publickey@o1.ingest.us.sentry.io/3", release: "" })
        .release,
    ).toBe("");
  });

  it("passes the dsn through", () => {
    expect(serverBuilt().dsn).toBe("https://publickey@o1.ingest.us.sentry.io/3");
  });

  it("carries no browser-only setting", () => {
    const built = serverBuilt() as ServerInitOptions & Record<string, unknown>;

    expect(built.replaysSessionSampleRate).toBeUndefined();
    expect(built.replaysOnErrorSampleRate).toBeUndefined();
    expect(built.denyUrls).toBeUndefined();
  });
});

describe("the two builders cannot drift", () => {
  it("produces deeply equal dataCollection objects", () => {
    expect(serverBuilt().dataCollection).toEqual(options().dataCollection);
  });

  it("locks every category on both, including graphQL and genAI", () => {
    // Deep equality alone is near-tautological while both builders spread one
    // shared object: a permissive-but-identical set would pass it.
    expect(options().dataCollection).toEqual(LOCKED_DATA_COLLECTION);
    expect(serverBuilt().dataCollection).toEqual(LOCKED_DATA_COLLECTION);
  });
});

describe("the server beforeSend chain", () => {
  it("sends an ordinary event", () => {
    const event = errorEvent({ error_kind: "internal" });

    expect(serverBuilt().beforeSend(event, hintFor(new Error("boom")))).toBe(
      event,
    );
  });

  it("drops an event tagged expected", () => {
    expect(
      serverBuilt().beforeSend(
        errorEvent({ expected: "true" }),
        hintFor(new Error("a 422")),
      ),
    ).toBeNull();
  });

  it("keeps a fetch failure, because that rule is browser-only", () => {
    const event = errorEvent();

    expect(
      serverBuilt().beforeSend(event, hintFor(new TypeError("Failed to fetch"))),
    ).toBe(event);
  });

  it("reads no browser global", () => {
    // navigator is absent in the SSR process, so a shared chain would throw on
    // the first server event rather than report it.
    const navigatorDescriptor = Object.getOwnPropertyDescriptor(
      globalThis,
      "navigator",
    )!;
    const event = errorEvent();
    Reflect.deleteProperty(globalThis, "navigator");

    try {
      expect(serverBuilt().beforeSend(event, hintFor(new Error("boom")))).toBe(
        event,
      );
    } finally {
      Object.defineProperty(globalThis, "navigator", navigatorDescriptor);
    }
  });
});
