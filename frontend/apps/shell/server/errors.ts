import { reportError } from "@gofin/api";

/**
 * The error contract `server.js`'s production middleware consumes.
 *
 * `server.js` is the process entry and sits outside this bundle, so it cannot
 * import the workspace: `@gofin/api` ships TypeScript with no build output and
 * the runner image receives only its `package.json`. Both values below are
 * re-exported from `server/app.ts`, the SSR rollup input, so the middleware
 * reads them off the same `import(BUILD_PATH)` namespace that supplies `app`.
 */

/**
 * The body the production error middleware responds with. Frozen, because a
 * shared literal that reaches the wire should not be mutable by one consumer.
 * The shape is the API error envelope the client already parses.
 */
export const serverErrorBody: Readonly<{ code: string; message: string }> =
  Object.freeze({
    code: "INTERNAL_ERROR",
    message: "An unexpected error occurred",
  });

/**
 * Reports an error Express handed to the production error middleware. Calling
 * the SDK there instead would add a second capture site outside the taxonomy, on
 * the one surface whose events prove the server init ran early enough.
 */
export function reportServerError(error: unknown): void {
  reportError(error, {
    kind: "internal",
    op: "ssr.request",
    domain: "platform",
  });
}
