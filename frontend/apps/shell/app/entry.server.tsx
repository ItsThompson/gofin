import { createReadableStreamFromReadable } from "@react-router/node";
import * as Sentry from "@sentry/react-router";
import { renderToPipeableStream } from "react-dom/server";
import { ServerRouter } from "react-router";

/**
 * React Router reads this export to bound single-fetch data streaming, falling
 * back to 4950 ms when it is absent. It is the value the framework's own default
 * entry carried before this file existed.
 */
export const streamTimeout = 5_000;

/**
 * Required by React Router once this file exists: a file exporting only
 * handleError renders nothing. The SDK's factory wraps the framework's default
 * streaming render and wires the trace meta-tag transformer, so neither is
 * reimplemented here.
 *
 * This file does not call Sentry.init. The server init is instrument.server.mjs,
 * imported first by server.js, because this module is reached lazily through
 * virtual:react-router/server-build on the first request: an init here would run
 * after server/app.ts, server/healthz.ts and the server.js error middleware have
 * already had a chance to fail.
 *
 * Sentry.createSentryServerInstrumentation is deliberately not exported: it
 * creates tracing spans for loaders, actions and request handlers, and tracing
 * is off.
 */
const handleRequest = Sentry.createSentryHandleRequest({
  ServerRouter,
  renderToPipeableStream,
  createReadableStreamFromReadable,
});

export default handleRequest;

/**
 * The sole owner of SSR render and loader errors. The factory captures through
 * captureException with a mechanism and nothing else, so these events carry the
 * three constant tags and none of the helper's taxonomy, by design.
 *
 * logErrors stays false: the presence of a handleError export already disables
 * React Router's own console.error, so this only controls whether the SDK adds
 * a second one.
 */
export const handleError = Sentry.createSentryHandleError({ logErrors: false });
