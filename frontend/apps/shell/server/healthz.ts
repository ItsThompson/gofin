import type { RequestHandler } from "express";
import { reportError } from "@gofin/api";

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
    } catch (error) {
      // The gateway's own 503 is the try branch's happy path; reaching here means
      // the probe never got an answer, and until now the cause was discarded.
      reportError(error, {
        kind: "upstream",
        op: "healthz.probe",
        domain: "platform",
        data: { gatewayUrl },
      });
      // Byte-identical: CI and the external health-check workflow both grep it.
      res
        .status(503)
        .json({ status: "unhealthy", reason: "gateway_unreachable" });
    }
  };
