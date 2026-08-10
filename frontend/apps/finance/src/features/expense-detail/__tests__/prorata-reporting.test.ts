import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { createMockApi, buildExpense } from "@gofin/test-utils";

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

import { useExpenseDetail } from "../hooks/useExpenseDetail";

/** The single recorded capture, so every assertion also proves an exact count. */
function onlyCapture(): { error: unknown; context: CapturedContext } {
  expect(captureException).toHaveBeenCalledTimes(1);
  const [error, context] = captureException.mock.calls[0];
  expect(context).toBeDefined();
  return { error, context: context as CapturedContext };
}

const PRO_RATA_EXPENSE = buildExpense({
  id: "exp-1",
  isProRata: true,
  proRataGroup: "group-1",
  proRataIndex: 1,
  proRataTotal: 3,
});

/** Longest paths first: the mock router matches by substring. */
function mockApi(groupResponse: unknown): typeof fetch {
  return createMockApi({
    "/api/expenses/prorata/group-1": groupResponse,
    "/api/expenses/exp-1/history": { body: { entries: [] } },
    "/api/expenses/exp-1": { body: { expense: PRO_RATA_EXPENSE } },
  }) as unknown as typeof fetch;
}

describe("useExpenseDetail's installment list", () => {
  beforeEach(() => {
    captureException.mockClear();
    toastError.mockClear();
  });

  it("still shows the expense when the installment fetch fails", async () => {
    global.fetch = mockApi({ status: 503, body: { code: "UPSTREAM" } });

    const { result } = renderHook(() => useExpenseDetail("exp-1"));

    await waitFor(() => expect(result.current.status).toBe("detail"));
    expect(
      result.current.status === "detail" ? result.current.proRataGroup : null,
    ).toEqual([]);
  });

  it("reports the failure and tells the user, instead of dropping rows in silence", async () => {
    global.fetch = mockApi({ status: 503, body: { code: "UPSTREAM" } });

    renderHook(() => useExpenseDetail("exp-1"));

    await waitFor(() => expect(captureException).toHaveBeenCalled());

    expect(onlyCapture().context.tags).toMatchObject({
      error_kind: "upstream",
      operation: "expense.pro_rata_group",
      domain: "expenses",
    });
    expect(toastError).toHaveBeenCalledTimes(1);
  });

  it("reports nothing when the installment list loads", async () => {
    global.fetch = mockApi({
      body: { data: [PRO_RATA_EXPENSE], total: 1, page: 1, pageSize: 50, hasMore: false },
    });

    const { result } = renderHook(() => useExpenseDetail("exp-1"));

    await waitFor(() => expect(result.current.status).toBe("detail"));
    expect(captureException).not.toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
  });
});
