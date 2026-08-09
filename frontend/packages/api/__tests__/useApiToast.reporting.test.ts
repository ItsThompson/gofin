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
    () => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  ),
}));

vi.mock("@sentry/react-router", () => ({ captureException }));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));

import { toast } from "sonner";
import { ApiRequestError } from "../src/client";
import { useApiToast } from "../src/hooks/useApiToast";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

/** Invokes the Retry action attached to the most recent toast. */
function clickRetry(): void {
  const calls = (toast.error as ReturnType<typeof vi.fn>).mock.calls;
  const options = calls[calls.length - 1][1];
  options.action.onClick();
}

describe("useApiToast reporting", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("reports the caller's op and domain beside the toast", async () => {
    const { result } = renderHook(() =>
      useApiToast({ op: "expense.list", domain: "expenses" }),
    );
    const failure = new ApiRequestError(503, {
      code: "UNAVAILABLE",
      message: "Service unavailable",
    });

    await act(async () => {
      await result.current.call(() => Promise.reject(failure));
    });

    const { error, context } = onlyCapture();
    expect(error).toBe(failure);
    expect(context.tags).toEqual({
      error_kind: "upstream",
      operation: "expense.list",
      domain: "expenses",
    });
    expect(context.level).toBe("error");
    expect(toast.error).toHaveBeenCalledTimes(1);
  });

  it("puts the attempt count in the gofin context block", async () => {
    const { result } = renderHook(() => useApiToast());

    await act(async () => {
      await result.current.call(() => Promise.reject(new Error("boom")));
    });

    expect(onlyCapture().context.contexts).toEqual({ gofin: { attempt: 1 } });
  });

  it("reports once when one failure is followed by two Retry clicks", async () => {
    const operation = vi.fn(() => Promise.reject(new Error("still down")));
    const { result } = renderHook(() => useApiToast());

    await act(async () => {
      await result.current.call(operation);
    });
    await act(async () => {
      clickRetry();
    });
    await act(async () => {
      clickRetry();
    });

    expect(operation).toHaveBeenCalledTimes(3);
    expect(toast.error).toHaveBeenCalledTimes(3);
    expect(captureException).toHaveBeenCalledTimes(1);
  });

  it("reports again once a new operation starts a new chain", async () => {
    const { result } = renderHook(() => useApiToast());

    await act(async () => {
      await result.current.call(() => Promise.reject(new Error("first")));
    });
    await act(async () => {
      await result.current.call(() => Promise.reject(new Error("second")));
    });

    expect(captureException).toHaveBeenCalledTimes(2);
  });

  it("reports again after a success ends the chain", async () => {
    const operation = vi
      .fn<() => Promise<string>>()
      .mockRejectedValueOnce(new Error("down"))
      .mockResolvedValueOnce("recovered")
      .mockRejectedValueOnce(new Error("down again"));
    const { result } = renderHook(() => useApiToast<string>());

    await act(async () => {
      await result.current.call(operation);
    });
    await act(async () => {
      clickRetry();
    });
    await act(async () => {
      clickRetry();
    });

    expect(operation).toHaveBeenCalledTimes(3);
    expect(captureException).toHaveBeenCalledTimes(2);
  });

  it("reports nothing for a silent call", async () => {
    const { result } = renderHook(() => useApiToast());

    await act(async () => {
      await result.current.call(() => Promise.reject(new Error("quiet")), {
        silent: true,
      });
    });

    expect(captureException).not.toHaveBeenCalled();
  });

  it("reports nothing for callSilent", async () => {
    const { result } = renderHook(() => useApiToast());

    await act(async () => {
      await result.current.callSilent(() => Promise.reject(new Error("quiet")));
    });

    expect(captureException).not.toHaveBeenCalled();
  });

  it("keeps a silent call out of the chain a visible call owns", async () => {
    const operation = vi.fn(() => Promise.reject(new Error("down")));
    const { result } = renderHook(() => useApiToast());

    await act(async () => {
      await result.current.call(operation);
    });
    await act(async () => {
      await result.current.call(operation, { silent: true });
    });
    await act(async () => {
      clickRetry();
    });

    expect(captureException).toHaveBeenCalledTimes(1);
  });

  describe("concurrent calls on one hook instance", () => {
    // useDashboardData fans out four toastCalls on one instance inside a single
    // Promise.all, so every in-flight call has to decide its own outcome.
    it("reports the failure when a sibling call succeeds first", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await Promise.all([
          result.current.call(() => Promise.resolve(undefined)),
          result.current.call(() => Promise.reject(new Error("one down"))),
        ]);
      });

      const { error } = onlyCapture();
      expect((error as Error).message).toBe("one down");
    });

    it("reports every failure in a partly failing fan-out", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await Promise.all([
          result.current.call(() => Promise.resolve(undefined)),
          result.current.call(() => Promise.reject(new Error("first down"))),
          result.current.call(() => Promise.reject(new Error("second down"))),
          result.current.call(() => Promise.reject(new Error("third down"))),
        ]);
      });

      expect(captureException).toHaveBeenCalledTimes(3);
    });

    it("reports once per operation when the whole fan-out fails", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await Promise.all([
          result.current.call(() => Promise.reject(new Error("a"))),
          result.current.call(() => Promise.reject(new Error("b"))),
          result.current.call(() => Promise.reject(new Error("c"))),
          result.current.call(() => Promise.reject(new Error("d"))),
        ]);
      });

      expect(captureException).toHaveBeenCalledTimes(4);
    });
  });

  it("tags a network failure as network at warning level", async () => {
    const { result } = renderHook(() => useApiToast({ op: "expense.list" }));

    await act(async () => {
      await result.current.call(() =>
        Promise.reject(new TypeError("Failed to fetch")),
      );
    });

    const { context } = onlyCapture();
    expect(context.tags).toEqual({
      error_kind: "network",
      operation: "expense.list",
    });
    expect(context.level).toBe("warning");
  });

  it("tags a sub-500 status expected as the string \"true\"", async () => {
    const { result } = renderHook(() => useApiToast());

    await act(async () => {
      await result.current.call(() =>
        Promise.reject(
          new ApiRequestError(401, { code: "UNAUTHORIZED", message: "Expired" }),
        ),
      );
    });

    const { context } = onlyCapture();
    expect(context.tags?.expected).toBe("true");
    expect(typeof context.tags?.expected).toBe("string");
    expect(context.tags?.error_kind).toBe("validation");
  });

  it("does not tag a 5xx expected", async () => {
    const { result } = renderHook(() => useApiToast());

    await act(async () => {
      await result.current.call(() =>
        Promise.reject(
          new ApiRequestError(500, { code: "INTERNAL", message: "Boom" }),
        ),
      );
    });

    expect(onlyCapture().context.tags).not.toHaveProperty("expected");
  });
});
