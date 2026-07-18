import type { RequestHandler } from "express";

/**
 * Dependencies for {@link createHealthzHandler}. Injected so the handler is unit
 * testable without booting SSR or a real gateway.
 */
interface HealthzDeps {
  gatewayUrl: string;
  fetchFn: typeof fetch;
  timeoutMs: number;
}

/**
 * createHealthzHandler builds the shell's public /healthz liveness handler.
 *
 * It mirrors the gateway's /readyz aggregate: 200 when the gateway reports every
 * backend service healthy, 503 otherwise. The gateway's JSON body is relayed
 * verbatim so CI logs show which downstream service failed. A hung or
 * unreachable gateway is bounded by AbortSignal.timeout, so the endpoint always
 * responds (503) rather than hanging.
 */
export const createHealthzHandler =
  ({ gatewayUrl, fetchFn, timeoutMs }: HealthzDeps): RequestHandler =>
  async (_req, res) => {
    try {
      const response = await fetchFn(`${gatewayUrl}/readyz`, {
        signal: AbortSignal.timeout(timeoutMs),
      });
      const body = await response.text();
      res
        .status(response.ok ? 200 : 503)
        .type("application/json")
        .send(body);
    } catch {
      res
        .status(503)
        .json({ status: "unhealthy", reason: "gateway_unreachable" });
    }
  };
