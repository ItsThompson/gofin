import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { createMockApi } from "@gofin/test-utils";

/** The subset of Sentry's CaptureContext that reportError sends. */
interface CapturedContext {
  level?: string;
  tags?: Record<string, string>;
  fingerprint?: string[];
  contexts?: Record<string, Record<string, unknown>>;
}

const { captureException, toastError } = vi.hoisted(() => ({
  captureException: vi.fn<(error: unknown, context?: CapturedContext) => string>(
    () => "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  ),
  toastError: vi.fn(),
}));

vi.mock("@sentry/react-router", () => ({ captureException }));
vi.mock("sonner", () => ({
  toast: { error: toastError, success: vi.fn() },
}));

import { useAdminPanel } from "../hooks/useAdminPanel";
import { useUserDeletion } from "../hooks/useUserDeletion";
import { buildUser } from "@gofin/test-utils";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

beforeEach(() => {
  captureException.mockClear();
  toastError.mockClear();
  global.fetch = createMockApi({
    "/api/admin/users": { body: { users: [] } },
  }) as unknown as typeof fetch;
});

describe("a failed identity assumption", () => {
  function renderPanel(onAssumeIdentity: (userId: string) => Promise<void>) {
    return renderHook(() =>
      useAdminPanel({ currentUser: buildUser(), onAssumeIdentity }),
    );
  }

  it("reports, tells the operator, and clears the row's spinner", async () => {
    const failure = new Error("assume refused");
    const { result } = renderPanel(() => Promise.reject(failure));

    await waitFor(() => expect(result.current.state.loadState).toBe("success"));
    await act(async () => {
      await result.current.actions.handleAssume("user-2");
    });

    const capture = onlyCapture();
    expect(capture.error).toBe(failure);
    expect(capture.context.tags).toMatchObject({
      error_kind: "internal",
      operation: "auth.assume_identity",
      domain: "auth",
    });
    expect(capture.context.contexts?.gofin).toEqual({ userId: "user-2" });
    expect(toastError).toHaveBeenCalledTimes(1);
    expect(result.current.state.assumingUserId).toBeNull();
  });

  it("reports nothing when the assumption succeeds", async () => {
    const { result } = renderPanel(() => Promise.resolve());

    await waitFor(() => expect(result.current.state.loadState).toBe("success"));
    await act(async () => {
      await result.current.actions.handleAssume("user-2");
    });

    expect(captureException).not.toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
  });
});

describe("a deletion status poll that gives up", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("reports once, from the caller, and keeps the row's last known status", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    global.fetch = vi
      .fn()
      .mockRejectedValue(new TypeError("Failed to fetch")) as unknown as typeof fetch;

    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: vi.fn() }),
    );

    act(() => {
      result.current.actions.handleDeletionSuccess({
        id: "job-1",
        userId: "user-2",
        status: "pending",
      } as never);
    });

    // Three consecutive failures exhaust the budget. The extra ticks would each
    // add an event if the transport layer reported per failure.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60000);
    });

    const { context } = onlyCapture();
    expect(context.tags).toMatchObject({
      error_kind: "network",
      operation: "datarights.deletion_status",
      domain: "datarights",
    });
    expect(context.tags?.expected).toBeUndefined();
    expect(context.contexts?.gofin).toEqual({ jobId: "job-1" });
    expect(toastError).toHaveBeenCalledTimes(1);
    // Unknown outcome, so the row keeps what it last knew.
    expect(result.current.state.deletionStates["user-2"]?.status).toBe("pending");
  });
});
