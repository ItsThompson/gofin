import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildPeriod, createMockApi } from "@gofin/test-utils";
import { useDashboardData } from "../hooks/useDashboardData";
import { dashboardDataEmptyRoutes } from "./fixtures";

interface CapturedContext {
  tags?: Record<string, string>;
}

const { captureException, toastError } = vi.hoisted(() => ({
  captureException: vi.fn<(error: unknown, context?: CapturedContext) => string>(() => "event-id"),
  toastError: vi.fn(),
}));
vi.mock("@sentry/react-router", () => ({ captureException }));
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }));

const period = buildPeriod({ id: "period-1", year: 2026, month: 5 });

beforeEach(() => {
  captureException.mockClear();
  toastError.mockClear();
  window.matchMedia = vi.fn().mockImplementation(() => ({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
});

describe("dashboard period retry reporting", () => {
  it.each(["summary", "refresh"] as const)("reports a failed %s period lookup once", async (action) => {
    const mockApi = createMockApi({
      ...dashboardDataEmptyRoutes(),
      "/api/finance/health-score/trend": { body: { trends: [] } },
      "/api/finance/health-score": { body: { healthScore: { configureBudget: true } } },
      "/api/finance/periods/current": {
        status: 503,
        body: { code: "UNAVAILABLE", message: "Period lookup failed" },
      },
    });
    globalThis.fetch = mockApi as unknown as typeof fetch;
    const { result } = renderHook(() => useDashboardData(period));
    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));

    act(() => {
      if (action === "summary") result.current.retry("summary");
      else result.current.refresh();
    });
    await waitFor(() => expect(result.current.periodStatus).toBe("error"));
    expect(captureException).toHaveBeenCalledTimes(1);
    expect(captureException.mock.calls[0][1]?.tags).toMatchObject({
      error_kind: "upstream",
      operation: "budget.period",
      domain: "budgets",
    });
  });

  it("reports section failures without showing an error toast", async () => {
    global.fetch = createMockApi({
      ...dashboardDataEmptyRoutes(),
      "/api/finance/health-score": {
        status: 503,
        body: { code: "UPSTREAM_UNAVAILABLE", message: "Health score failed" },
      },
    }) as unknown as typeof fetch;

    const { result } = renderHook(() => useDashboardData(period));
    await waitFor(() => expect(result.current.sections.healthScore.status).toBe("error"));

    expect(toastError).not.toHaveBeenCalled();
    expect(captureException).toHaveBeenCalled();
    expect(captureException.mock.calls.some(([, context]) =>
      context?.tags?.operation === "dashboard.healthScore" &&
      context.tags?.error_kind === "upstream" &&
      context.tags?.domain === "budgets",
    )).toBe(true);
  });

  it("does not report a period failure after the dashboard unmounts", async () => {
    let rejectPeriod!: (error: Error) => void;
    const mockApi = createMockApi({
      ...dashboardDataEmptyRoutes(),
      "/api/finance/health-score/trend": { body: { trends: [] } },
      "/api/finance/health-score": { body: { healthScore: { configureBudget: true } } },
    });
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).includes("/periods/current")) {
        return new Promise<Response>((_resolve, reject) => { rejectPeriod = reject; });
      }
      return mockApi(input, init);
    }) as typeof fetch;
    const { result, unmount } = renderHook(() => useDashboardData(period));
    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));
    act(() => result.current.retry("summary"));
    unmount();
    await act(async () => rejectPeriod(new Error("Late period failure")));
    expect(captureException).not.toHaveBeenCalled();
  });
});
