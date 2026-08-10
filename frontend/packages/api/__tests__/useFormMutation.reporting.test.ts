import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

/** The subset of Sentry's CaptureContext that reportError sends. */
interface CapturedContext {
  level?: string;
  tags?: Record<string, string>;
  fingerprint?: string[];
  contexts?: Record<string, Record<string, unknown>>;
}

const { captureException } = vi.hoisted(() => ({
  captureException: vi.fn<(error: unknown, context?: CapturedContext) => string>(
    () => "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  ),
}));

vi.mock("@sentry/react-router", () => ({ captureException }));

import { ApiRequestError } from "../src/client";
import { useFormMutation } from "../src/hooks/useFormMutation";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

async function submitRejecting(
  failure: unknown,
  options?: Parameters<typeof useFormMutation<void>>[0],
): Promise<void> {
  const { result } = renderHook(() => useFormMutation<void>(options));
  await act(async () => {
    result.current.submit(() => Promise.reject(failure));
  });
}

describe("useFormMutation reporting", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("reports a 5xx at error level with kind upstream", async () => {
    const failure = new ApiRequestError(500, {
      code: "INTERNAL_ERROR",
      message: "Server error",
    });

    await submitRejecting(failure, {
      op: "expense.create",
      domain: "expenses",
    });

    const { error, context } = onlyCapture();
    expect(error).toBe(failure);
    expect(context.level).toBe("error");
    expect(context.tags).toEqual({
      error_kind: "upstream",
      operation: "expense.create",
      domain: "expenses",
      http_status: "500",
    });
    expect(context.tags).not.toHaveProperty("expected");
  });

  it("reports a 4xx at warning level tagged expected as the string \"true\"", async () => {
    await submitRejecting(
      new ApiRequestError(422, {
        code: "VALIDATION_ERROR",
        message: "Amount must be positive",
        fields: { amount: "must be positive" },
      }),
      { op: "expense.create", domain: "expenses" },
    );

    const { context } = onlyCapture();
    expect(context.level).toBe("warning");
    expect(context.tags?.expected).toBe("true");
    expect(typeof context.tags?.expected).toBe("string");
    expect(context.tags?.error_kind).toBe("validation");
    expect(context.tags?.http_status).toBe("422");
  });

  it("puts field names in the gofin context block and never their values", async () => {
    await submitRejecting(
      new ApiRequestError(422, {
        code: "VALIDATION_ERROR",
        message: "Check the form",
        fields: { amount: "must be positive", merchant: "is required" },
      }),
    );

    const { context } = onlyCapture();
    expect(context.contexts).toEqual({
      gofin: { fields: ["amount", "merchant"] },
    });
    expect(JSON.stringify(context)).not.toContain("must be positive");
    expect(JSON.stringify(context)).not.toContain("is required");
  });

  it("reports an empty field list when the error carries no fields", async () => {
    await submitRejecting(
      new ApiRequestError(409, { code: "CONFLICT", message: "Already exists" }),
    );

    expect(onlyCapture().context.contexts).toEqual({ gofin: { fields: [] } });
  });

  it("tags a network failure as network and sends no http_status", async () => {
    await submitRejecting(new TypeError("Load failed"));

    const { context } = onlyCapture();
    expect(context.tags).toEqual({ error_kind: "network" });
    expect(context.level).toBe("warning");
    expect(context.contexts).toBeUndefined();
  });

  it("reports an unrecognized failure as internal at error level", async () => {
    await submitRejecting(42);

    const { error, context } = onlyCapture();
    expect(error).toBe(42);
    expect(context.tags).toEqual({ error_kind: "internal" });
    expect(context.level).toBe("error");
  });

  it("reports once per submit", async () => {
    const { result } = renderHook(() => useFormMutation<void>());

    await act(async () => {
      result.current.submit(() => Promise.reject(new Error("first")));
    });
    await act(async () => {
      result.current.submit(() => Promise.reject(new Error("second")));
    });

    expect(captureException).toHaveBeenCalledTimes(2);
  });

  it("reports nothing on success", async () => {
    const { result } = renderHook(() => useFormMutation<string>());

    await act(async () => {
      result.current.submit(() => Promise.resolve("ok"));
    });

    expect(captureException).not.toHaveBeenCalled();
  });

  it("passes the original error to onError alongside the message", async () => {
    const onError = vi.fn();
    const failure = new ApiRequestError(500, {
      code: "INTERNAL_ERROR",
      message: "Server error",
    });

    await submitRejecting(failure, { onError });

    expect(onError).toHaveBeenCalledWith("Server error", failure);
  });
});
