import { createReadableStreamFromReadable } from "@react-router/node";
import * as Sentry from "@sentry/react-router";
import { renderToPipeableStream } from "react-dom/server";
import { isRouteErrorResponse, ServerRouter } from "react-router";

/**
 * React Router reads this export to bound single-fetch data streaming, and falls
 * back to 4950 ms when it is absent.
 */
export const streamTimeout = 5_000;

/**
 * The SDK's factory wraps the framework's default streaming render and wires the
 * trace meta-tag transformer, so neither is reimplemented here.
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
const sentryHandleRequest = Sentry.createSentryHandleRequest({
  ServerRouter,
  renderToPipeableStream,
  createReadableStreamFromReadable,
  // The factory defaults to 10 s. Aborting the render one second after the data
  // stream's own budget is the relationship the framework's default entry holds.
  streamTimeout: streamTimeout + 1000,
});

/**
 * Required by React Router once this file exists: a module exporting only
 * handleError renders nothing.
 *
 * A HEAD response carries no body (RFC 9110 section 9.3.2) and the factory has no
 * short-circuit for it, so the guard sits in front of the factory rather than
 * inside a reimplemented render. Crawlers and uptime monitors send HEAD to the
 * public marketing route, and rendering a document to discard it is waste.
 *
 * The factory's type is an intersection of two signatures that differ only in the
 * load-context shape. The wrapper forwards every argument untouched, so one
 * assertion at the end is honest and avoids restating both signatures.
 */
const handleRequest = ((...args: Parameters<typeof sentryHandleRequest>) => {
  const [request, responseStatusCode, responseHeaders] = args;
  if (request.method.toUpperCase() === "HEAD") {
    return new Response(null, {
      status: responseStatusCode,
      headers: responseHeaders,
    });
  }
  return sentryHandleRequest(...args);
}) as typeof sentryHandleRequest;

export default handleRequest;

const sentryHandleError = Sentry.createSentryHandleError({ logErrors: false });

/**
 * The sole owner of SSR render and loader errors. The factory captures through
 * the SDK with a mechanism and nothing else, so these events carry the three
 * constant tags and none of the helper's taxonomy, by design.
 *
 * logErrors stays false: the presence of a handleError export already disables
 * React Router's own console.error, so this only controls whether the SDK adds
 * a second one.
 *
 * React Router hands this hook the errors it generates for a request it could
 * not route, not only application failures: an unmatched URL is a 404 and every
 * browser asks for /favicon.ico, and a POST to a route with no action is a 405.
 * Those are client-fault protocol outcomes rather than defects. Sub-500 is where
 * this codebase already draws that line, in classifyApiFailure and in the Go
 * capture rule, so an ErrorResponse below 500 is not captured here either. A 5xx
 * one still is: a loader signalling upstream failure is a real event.
 */
export const handleError: typeof sentryHandleError = async (error, args) => {
  if (isRouteErrorResponse(error) && error.status < 500) return;
  await sentryHandleError(error, args);
};
