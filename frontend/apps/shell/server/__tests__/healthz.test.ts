import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Response } from "express";

/** The subset of Sentry's CaptureContext that reportError sends. */
interface CapturedContext {
  level?: string;
  tags?: Record<string, string>;
  fingerprint?: string[];
  contexts?: Record<string, Record<string, unknown>>;
}

const { captureException } = vi.hoisted(() => ({
  captureException: vi.fn<(error: unknown, context?: CapturedContext) => string>(
    () => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  ),
}));

vi.mock("@sentry/react-router", () => ({ captureException }));

import { createHealthzHandler } from "../healthz";

const GATEWAY_URL = "http://gateway:8080";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

// Captures what the handler wrote to the express Response. The handler only
// touches status/type/send/json, so a minimal chainable stub stands in for the
// full express Response (one boundary cast).
interface CapturedResponse {
  status: number;
  contentType?: string;
  body?: string | Record<string, unknown>;
}

function createMockRes(captured: CapturedResponse): Response {
  const res = {
    status(code: number) {
      captured.status = code;
      return res;
    },
    type(value: string) {
      captured.contentType = value;
      return res;
    },
    send(payload: string) {
      captured.body = payload;
      return res;
    },
    json(payload: Record<string, unknown>) {
      captured.body = payload;
      return res;
    },
  };
  return res as unknown as Response;
}

async function invokeHandler(fetchFn: typeof fetch): Promise<CapturedResponse> {
  const captured: CapturedResponse = { status: 0 };
  const handler = createHealthzHandler({
    gatewayUrl: GATEWAY_URL,
    fetchFn,
    timeoutMs: 5000,
  });
  // The handler ignores the request; a bare object stands in for it.
  await handler(
    {} as never,
    createMockRes(captured),
    (() => {}) as never,
  );
  return captured;
}

describe("createHealthzHandler", () => {
  beforeEach(() => {
    captureException.mockClear();
  });

  it("relays 200 and the gateway body when /readyz is healthy", async () => {
    const fetchFn = vi.fn(async () => ({
      ok: true,
      status: 200,
      text: async () => '{"status":"ok"}',
    })) as unknown as typeof fetch;

    const captured = await invokeHandler(fetchFn);

    expect(fetchFn).toHaveBeenCalledWith(
      `${GATEWAY_URL}/readyz`,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(captured.status).toBe(200);
    expect(captured.contentType).toBe("application/json");
    expect(captured.body).toBe('{"status":"ok"}');
    expect(captureException).not.toHaveBeenCalled();
  });

  it("relays 503 and the gateway body naming the failing service", async () => {
    const gatewayBody =
      '{"status":"unhealthy","services":{"expense":"unreachable"}}';
    const fetchFn = vi.fn(async () => ({
      ok: false,
      status: 503,
      text: async () => gatewayBody,
    })) as unknown as typeof fetch;

    const captured = await invokeHandler(fetchFn);

    expect(captured.status).toBe(503);
    expect(captured.contentType).toBe("application/json");
    expect(captured.body).toBe(gatewayBody);
    // An unhealthy gateway answered the probe. The failing service is already
    // named in the relayed body and owned by the backend's own reporting.
    expect(captureException).not.toHaveBeenCalled();
  });

  it("returns 503 gateway_unreachable when fetch rejects", async () => {
    const fetchFn = vi.fn(async () => {
      throw new Error("ECONNREFUSED");
    }) as unknown as typeof fetch;

    const captured = await invokeHandler(fetchFn);

    expect(captured.status).toBe(503);
    expect(captured.body).toEqual({
      status: "unhealthy",
      reason: "gateway_unreachable",
    });
  });

  it("reports the gateway error it used to discard", async () => {
    const cause = new Error("ECONNREFUSED");
    const fetchFn = vi.fn(async () => {
      throw cause;
    }) as unknown as typeof fetch;

    await invokeHandler(fetchFn);

    const { error, context } = onlyCapture();
    expect(error).toBe(cause);
    expect(context.tags).toMatchObject({
      error_kind: "upstream",
      operation: "healthz.probe",
      domain: "platform",
    });
    expect(context.contexts?.gofin).toEqual({ gatewayUrl: GATEWAY_URL });
  });

  it("reports a timeout too, and still answers", async () => {
    const cause = new DOMException("The operation timed out.", "TimeoutError");
    const fetchFn = vi.fn(async () => {
      throw cause;
    }) as unknown as typeof fetch;

    const captured = await invokeHandler(fetchFn);

    expect(onlyCapture().error).toBe(cause);
    expect(captured.status).toBe(503);
  });

  it("returns 503 gateway_unreachable when the request times out", async () => {
    // AbortSignal.timeout aborts with a DOMException; the handler must treat a
    // timeout the same as any other failure and never hang.
    const fetchFn = vi.fn(async () => {
      throw new DOMException("The operation timed out.", "TimeoutError");
    }) as unknown as typeof fetch;

    const captured = await invokeHandler(fetchFn);

    expect(captured.status).toBe(503);
    expect(captured.body).toEqual({
      status: "unhealthy",
      reason: "gateway_unreachable",
    });
  });
});
