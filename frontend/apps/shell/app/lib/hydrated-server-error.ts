/** Reads the hydration payload React Router writes for an SSR route error. */

interface SerializedError {
  __type?: string;
  message?: string;
}

interface HydrationContext {
  state?: { errors?: Record<string, SerializedError | null> | null };
}

/**
 * Whether a route error the boundary received is the one the server already
 * reported, arriving back on the client through the hydration payload.
 *
 * React Router serializes an SSR route error into `window.__reactRouterContext`
 * and rebuilds it during hydration, so the client boundary renders for an error
 * that never happened in the browser. Effects do not run on the server, but they
 * do run on the way back, so "effects do not run during SSR" is not on its own
 * enough to keep `entry.server.tsx`'s `handleError` the sole owner of that event.
 *
 * The match reads the payload React Router actually writes: an `Error` is
 * serialized as `{ __type: "Error", message }`, so an entry of that shape whose
 * message equals the error's message identifies the server's error. Identity
 * cannot be used, because the client rebuilds a new `Error` from the payload
 * rather than reusing what the stream decoded.
 *
 * Suppressing here is only safe because anything it matches is provably already
 * owned: React Router sanitizes every server-side `Error` to the single message
 * "Unexpected Server Error" in production, so no client-side error can produce
 * one, and every server-side error reaches `handleError`.
 *
 * **Precondition: no `loader` and no `action` anywhere in `apps/shell`.** This
 * reads the initial document's payload only. A server-side error raised during a
 * client-side single-fetch navigation is captured by `handleError` too, but never
 * reaches this payload, so the effect would report it a second time. Adding a
 * `loader` or an `action` therefore requires revisiting this guard.
 */
export function isHydratedServerError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;

  const context = (globalThis as { __reactRouterContext?: HydrationContext })
    .__reactRouterContext;
  const serialized = context?.state?.errors;
  if (!serialized) return false;

  return Object.values(serialized).some(
    (entry) => entry?.__type === "Error" && entry.message === error.message,
  );
}
