import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { createMockApi, buildDefaults } from "@gofin/test-utils";

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

import { usePeriodState } from "../hooks/usePeriodState";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

const PERIOD_MISSING = {
  status: 404,
  body: { code: "PERIOD_NOT_FOUND", message: "no period for this month" },
};

describe("usePeriodState's saved defaults", () => {
  beforeEach(() => {
    captureException.mockClear();
    toastError.mockClear();
  });

  it("still offers the create prompt when the defaults fetch fails", async () => {
    global.fetch = createMockApi({
      "/api/finance/periods/current": PERIOD_MISSING,
      "/api/finance/defaults": { status: 503, body: { code: "UPSTREAM" } },
    }) as unknown as typeof fetch;

    const { result } = renderHook(() => usePeriodState());

    await waitFor(() => expect(result.current.status).toBe("no-period"));
    expect(result.current).toMatchObject({ status: "no-period", defaults: null });
  });

  it("reports the failure and tells the user, instead of looking like no defaults", async () => {
    global.fetch = createMockApi({
      "/api/finance/periods/current": PERIOD_MISSING,
      "/api/finance/defaults": { status: 503, body: { code: "UPSTREAM" } },
    }) as unknown as typeof fetch;

    renderHook(() => usePeriodState());

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    expect(onlyCapture().context.tags).toMatchObject({
      error_kind: "upstream",
      operation: "budget.defaults",
      domain: "budgets",
    });
    expect(toastError).toHaveBeenCalledTimes(1);
  });

  it("reports nothing when the defaults load", async () => {
    global.fetch = createMockApi({
      "/api/finance/periods/current": PERIOD_MISSING,
      "/api/finance/defaults": { body: { defaults: buildDefaults() } },
    }) as unknown as typeof fetch;

    const { result } = renderHook(() => usePeriodState());

    await waitFor(() => expect(result.current.status).toBe("no-period"));
    expect(captureException).not.toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
  });
});
