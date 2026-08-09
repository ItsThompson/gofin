import { describe, it, expect, vi, beforeEach } from "vitest";

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

import { reportServerError, serverErrorBody } from "../errors";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

describe("reportServerError", () => {
  beforeEach(() => {
    captureException.mockClear();
  });

  it("reports the error once, classified for this surface", () => {
    const error = new Error("proxy exploded");

    reportServerError(error);

    const capture = onlyCapture();
    expect(capture.error).toBe(error);
    expect(capture.context.tags).toMatchObject({
      error_kind: "internal",
      operation: "ssr.request",
      domain: "platform",
    });
  });

  it("does not mark the event expected, so beforeSend keeps it", () => {
    reportServerError(new Error("proxy exploded"));

    expect(onlyCapture().context.tags?.expected).toBeUndefined();
  });

  it("passes a non-Error value through untouched", () => {
    // reportError deliberately never wraps: a synthetic exception would root
    // every unrelated failure in one stack and collapse them into one issue.
    reportServerError("a string thrown by a dependency");

    expect(onlyCapture().error).toBe("a string thrown by a dependency");
  });
});

describe("serverErrorBody", () => {
  it("is the API error envelope the client already parses", () => {
    expect(serverErrorBody).toEqual({
      code: "INTERNAL_ERROR",
      message: "An unexpected error occurred",
    });
  });

  it("cannot be mutated by a consumer", () => {
    expect(Object.isFrozen(serverErrorBody)).toBe(true);
  });
});
