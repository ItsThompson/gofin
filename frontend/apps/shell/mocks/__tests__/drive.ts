import type { HttpHandler } from "msw";

/**
 * Resolve a path against a list of MSW handlers without a network layer.
 * Returns the first handler's mocked Response, mirroring how the worker picks a
 * handler at runtime (first match wins, in registration order).
 *
 * The request URL is built from `location.href` so its origin matches the base
 * MSW normalizes relative handler paths against in the jsdom test environment.
 */
export async function resolveMockRequest(
  handlers: HttpHandler[],
  path: string,
  init?: RequestInit,
): Promise<Response> {
  const request = new Request(new URL(path, location.href), init);
  const requestId = crypto.randomUUID();
  for (const handler of handlers) {
    const result = await handler.run({ request, requestId });
    if (result?.response) return result.response;
  }
  throw new Error(`No mock handler matched ${request.method} ${path}`);
}
