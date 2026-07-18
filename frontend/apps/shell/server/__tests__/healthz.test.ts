import { describe, it, expect, vi } from "vitest";
import type { Response } from "express";
import { createHealthzHandler } from "../healthz";

const GATEWAY_URL = "http://gateway:8080";

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
