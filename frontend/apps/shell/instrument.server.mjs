/**
 * The only server-side `Sentry.init` call site.
 *
 * `server.js` imports this as its first statement. That placement is
 * load-order critical: `server.js` is the process entry, the SSR bundle is
 * loaded after it, and `app/entry.server.tsx` is reached lazily on the first
 * request, so an init inside the bundle would run after `server/app.ts`,
 * `server/healthz.ts` and the `server.js` error middleware have already had a
 * chance to fail. An uninitialized client makes every report a silent no-op,
 * which reads exactly like "nothing crashed".
 *
 * Plain JavaScript beside `sentry.options.mjs`, because nothing compiles this
 * graph and the runner image copies only `apps/shell/build`, `server.js` and
 * these two files.
 *
 * `NODE_OPTIONS='--import ./instrument.server.mjs'` is deliberately not used.
 * It exists to enable OpenTelemetry auto-instrumentation, which only produces
 * spans, and Sentry supports that loader path only below Node 20.19 and 22.12
 * while every image stage is node:24-alpine. Error capture is registered by
 * `Sentry.init` itself.
 */
import * as Sentry from "@sentry/react-router";
import { serverOptions } from "./sentry.options.mjs";

const RELEASE_PREFIX = "gofin-web@";

const dsn = process.env.SENTRY_DSN;
const sha = process.env.SENTRY_RELEASE;

// DSN presence is the only switch, as in the browser entry: the E2E stack runs
// as production by design, so NODE_ENV cannot discriminate.
if (dsn) {
  Sentry.init(
    serverOptions({
      dsn,
      // SENTRY_RELEASE carries a bare SHA, and this is the one place the prefix
      // is applied: serverOptions uses the string verbatim, so the release
      // cannot become gofin-web@gofin-web@<sha>.
      release: sha ? `${RELEASE_PREFIX}${sha}` : undefined,
    }),
  );
}
