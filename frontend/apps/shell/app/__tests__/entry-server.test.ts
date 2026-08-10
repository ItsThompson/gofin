import { describe, it, expect, vi, beforeEach } from "vitest";

const { captureException, createSentryHandleRequest } = vi.hoisted(() => ({
  captureException: vi.fn<(error: unknown) => string>(
    () => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  ),
  createSentryHandleRequest: vi.fn<(options: Record<string, unknown>) => void>(),
}));

// Only the two factories are stubbed, and each keeps the shape the real one
// returns: the assertions below are about this module's exports and its two
// guards, which are the parts that are ours.
vi.mock("@sentry/react-router", () => ({
  createSentryHandleRequest: (options: Record<string, unknown>) => {
    createSentryHandleRequest(options);
    return vi.fn(() => new Response("rendered document"));
  },
  createSentryHandleError:
    ({ logErrors }: { logErrors: boolean }) =>
    async (error: unknown) => {
      captureException(error);
      if (logErrors) console.error(error);
    },
}));

import * as entryServer from "../entry.server";

const handleRequest = entryServer.default;

function requestArgs(method: string) {
  return [
    new Request("https://usegofin.com/", { method }),
    200,
    new Headers({ "x-probe": "1" }),
    {} as never,
    {} as never,
  ] as const;
}

/** The shape React Router hands handleError for an error it generated itself. */
function routeErrorResponse(status: number) {
  return {
    status,
    statusText: "Generated",
    data: `Error: ${status}`,
    internal: true,
  };
}

describe("the entry.server module contract", () => {
  it("keeps a callable default export", () => {
    // React Router requires it once the file exists, and turbo build does not
    // catch its absence: SSR fails at request time instead.
    expect(typeof entryServer.default).toBe("function");
  });

  it("exports handleError", () => {
    expect(typeof entryServer.handleError).toBe("function");
  });

  it("exports the stream timeout React Router reads", () => {
    expect(entryServer.streamTimeout).toBe(5_000);
  });

  it("exports no server instrumentation, because tracing is off", () => {
    expect(Object.keys(entryServer).sort()).toEqual([
      "default",
      "handleError",
      "streamTimeout",
    ]);
  });

  it("gives the factory a render abort one second past the data-stream budget", () => {
    expect(createSentryHandleRequest).toHaveBeenCalledWith(
      expect.objectContaining({ streamTimeout: 6_000 }),
    );
  });
});

describe("handleRequest", () => {
  it("answers a HEAD request without rendering a document", async () => {
    const response = await handleRequest(...requestArgs("HEAD"));

    expect(response).toBeInstanceOf(Response);
    expect((response as Response).status).toBe(200);
    expect((response as Response).headers.get("x-probe")).toBe("1");
    expect(await (response as Response).text()).toBe("");
  });

  it("renders a GET request through the factory", async () => {
    const response = await handleRequest(...requestArgs("GET"));

    expect(await (response as Response).text()).toBe("rendered document");
  });
});

describe("handleError", () => {
  const args = { request: new Request("https://usegofin.com/") } as never;

  beforeEach(() => {
    captureException.mockClear();
  });

  it("captures an application error", async () => {
    const error = new Error("loader exploded");

    await entryServer.handleError(error, args);

    expect(captureException).toHaveBeenCalledTimes(1);
    expect(captureException).toHaveBeenCalledWith(error);
  });

  it("captures a 5xx route error response", async () => {
    await entryServer.handleError(routeErrorResponse(503), args);

    expect(captureException).toHaveBeenCalledTimes(1);
  });

  it("drops a 404, because every browser asks for /favicon.ico", async () => {
    await entryServer.handleError(routeErrorResponse(404), args);

    expect(captureException).not.toHaveBeenCalled();
  });

  it("drops a 405, which is what a stray POST to any route produces", async () => {
    await entryServer.handleError(routeErrorResponse(405), args);

    expect(captureException).not.toHaveBeenCalled();
  });
});
