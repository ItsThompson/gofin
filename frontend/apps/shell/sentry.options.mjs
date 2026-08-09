/**
 * Sentry options for both frontend runtimes: the browser bundle and the SSR
 * Node process.
 *
 * Plain JavaScript at the app root rather than TypeScript under `app/`, because
 * the SSR process entry is outside the Vite bundle: nothing compiles that graph
 * and the runner image copies only `apps/shell/build`, `server.js`, and the two
 * root `.mjs` files. A specifier resolving into `app/` throws
 * ERR_MODULE_NOT_FOUND on the first import of the process entry.
 *
 * `@sentry/react-router` is the only import this module may take. It is a
 * `dependencies` entry, so it resolves in the runner's `node_modules` as well as
 * in the bundle. Everything from `@gofin/*` ships as TypeScript with no build
 * output and never reaches the runner image, so a value like `isNetworkError` is
 * passed in by the caller instead of imported here.
 */
import { dedupeIntegration } from "@sentry/react-router";

/** A Sentry environment can only be hidden, never deleted, so there is one. */
const ENVIRONMENT = "production";

/**
 * gofin stores personal financial data and every category below defaults to
 * collecting content, so the whole surface is locked rather than the parts that
 * look risky today. `graphQL` and `genAI` are inert while no integration enables
 * them, and are locked so enabling one later cannot start sending operation text
 * or prompt content. `frameContextLines` is deliberately left at its default:
 * source context is code, not user data.
 */
const DATA_COLLECTION = {
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

const IGNORE_ERRORS = [
  // recharts' ResponsiveContainer emits this across seven widget files. It is
  // the most common junk event in a charting app.
  "ResizeObserver loop completed with undelivered notifications",
];

/**
 * Origins that are not first-party code. The app loads no third-party scripts,
 * so the reachable non-first-party frames come from extensions injected into the
 * page and from pages opened off the filesystem.
 */
const DENY_URLS = [
  /^chrome-extension:\/\//i,
  /^chrome:\/\//i,
  /^moz-extension:\/\//i,
  /^safari-extension:\/\//i,
  /^safari-web-extension:\/\//i,
  /^extension:\/\//i,
  /^file:\/\//i,
];

/** `runtime` is added per runtime; `app` and `service` are constants. */
const CONSTANT_TAGS = { app: "gofin-web", service: "web" };

/**
 * Runtime-invariant settings. Both builders spread this, so the PII lockdown
 * cannot differ between the browser and the SSR process.
 *
 * Every key here must exist in `@sentry/core`'s `ClientOptions`, because the
 * object is spread into a browser client and a node client. The two Session
 * Replay sample rates live in `BrowserClientReplayOptions`, which `@sentry/node`
 * does not have, so they are client-only settings below.
 *
 * `dsn`, `release`, and `beforeSend` are per-runtime and deliberately absent.
 */
export const SHARED_OPTIONS = {
  environment: ENVIRONMENT,
  dataCollection: DATA_COLLECTION,
  tracesSampleRate: 0,
  ignoreErrors: IGNORE_ERRORS,
  // Already a @sentry/browser default and not a @sentry/node one, so this is a
  // no-op on the client and a real addition on the server. Passing the array
  // merges with the defaults rather than replacing them.
  integrations: [dedupeIntegration()],
};

/**
 * Browser options. `release` arrives already prefixed as `gofin-web@<sha>` from
 * the build arg that also keys the uploaded source maps, and is used verbatim:
 * prefixing again would split one deploy across two release names.
 *
 * `isNetworkError` is a parameter rather than an import for the reason given at
 * the top of this file.
 */
export function clientOptions({ dsn, release, isNetworkError }) {
  return {
    ...SHARED_OPTIONS,
    dsn,
    release,
    // The JS SDK has no top-level `tags` option, unlike sentry-go's
    // ClientOptions.Tags. A `tags` key here would be silently ignored, and this
    // module is plain JS so nothing would type-check it.
    initialScope: { tags: { ...CONSTANT_TAGS, runtime: "browser" } },
    // Replay records the DOM, and a replay of a budget screen is a screen
    // recording of someone's finances.
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 0,
    denyUrls: DENY_URLS,
    beforeSend(event, hint) {
      if (navigator.webdriver === true) return null;
      // A strict string comparison. Tag values are typed Primitive, so a boolean
      // `true` is legal and nothing coerces it, which would leave the
      // highest-volume quota drop silently never firing.
      if (event.tags?.expected === "true") return null;
      if (isNetworkError(hint.originalException)) return null;
      return event;
    },
  };
}
