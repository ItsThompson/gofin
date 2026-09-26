import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildPeriod, buildPeriodSummary } from "@gofin/test-utils";
import { useDashboardData } from "../hooks/useDashboardData";

const period = buildPeriod({ id: "period-1", year: 2026, month: 5, budgetAmount: 300000 });
const summary = buildPeriodSummary({ periodId: period.id, year: period.year, month: period.month });

function response(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function installBaseApi(overrides: (url: string) => Response | Promise<Response> = () => response({})) {
  globalThis.fetch = ((input: RequestInfo | URL) => overrides(String(input))) as typeof fetch;
}

function installMobileViewport() {
  window.matchMedia = vi.fn().mockImplementation(() => ({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
}

describe("useDashboardData", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    installMobileViewport();
  });

  it("publishes a summary before a slower health request and does not fetch desktop sections on mobile", async () => {
    let healthRequested = false;
    const requestedUrls: string[] = [];
    installBaseApi((url) => {
      requestedUrls.push(url);
      if (url.includes("/summary")) return response({ summary });
      if (url.includes("/health-score?")) {
        healthRequested = true;
        return new Promise<Response>(() => {});
      }
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const { result } = renderHook(() => useDashboardData(period));

    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));
    expect(result.current.sections.summary).toEqual({ status: "success", data: summary });
    expect(result.current.sections.healthScore.status).toBe("loading");
    expect(healthRequested).toBe(true);
    expect(result.current.desktopVisible).toBe(false);

    expect(requestedUrls.some((url) => url.includes("spending/by-tag"))).toBe(false);
    expect(requestedUrls.some((url) => url.includes("spending/trends"))).toBe(false);
  });

  it("starts desktop requests when the viewport becomes eligible", async () => {
    let onViewportChange: ((event: MediaQueryListEvent) => void) | undefined;
    window.matchMedia = vi.fn().mockImplementation(() => ({
      matches: false,
      addEventListener: (_event: string, listener: (event: MediaQueryListEvent) => void) => {
        onViewportChange = listener;
      },
      removeEventListener: vi.fn(),
    }));
    const requestedUrls: string[] = [];
    installBaseApi((url) => {
      requestedUrls.push(url);
      if (url.includes("/summary")) return response({ summary });
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      if (url.includes("/spending/by-tag")) return response({ tagSpending: [] });
      if (url.includes("/spending/cumulative")) return response({ points: [] });
      if (url.includes("/spending/comparison")) return response({ comparison: {} });
      if (url.includes("/spending/trends")) return response({ trends: [] });
      return response({});
    });

    const { result } = renderHook(() => useDashboardData(period));
    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));
    expect(requestedUrls.some((url) => url.includes("spending/by-tag"))).toBe(false);

    act(() => onViewportChange?.({ matches: true } as MediaQueryListEvent));
    await waitFor(() => expect(requestedUrls.some((url) => url.includes("spending/by-tag"))).toBe(true));
  });

  it("propagates force refresh when retrying one section", async () => {
    const sectionHeaders: string[] = [];
    installBaseApi((url) => {
      if (url.includes("/summary")) return response({ summary });
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) {
        sectionHeaders.push("expenses");
        return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      }
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).includes("/expenses?")) {
        sectionHeaders.push(new Headers(init?.headers).get("Cache-Control") ?? "");
      }
      return originalFetch(input, init);
    }) as typeof fetch;

    const { result } = renderHook(() => useDashboardData(period));
    await waitFor(() => expect(result.current.sections.recentExpenses.status).toBe("empty"));
    act(() => result.current.retry("recentExpenses"));
    await waitFor(() => expect(sectionHeaders).toContain("no-cache"));
  });

  it("restores the current-period creation flow after a refresh 404", async () => {
    installBaseApi((url) => {
      if (url.includes("/periods/current")) return response({ code: "PERIOD_NOT_FOUND", message: "No period" }, 404);
      if (url.includes("/defaults")) return response({ defaults: null });
      if (url.includes("/summary")) return response({ summary });
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const { result } = renderHook(() => useDashboardData(period));
    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));
    act(() => result.current.refresh());
    await waitFor(() => expect(result.current.periodStatus).toBe("no-period"));
    expect(result.current.periodRecovery?.defaults).toBeNull();
  });

  it("reports a historical refresh 404 as not found without requesting defaults", async () => {
    const requestedUrls: string[] = [];
    installBaseApi((url) => {
      requestedUrls.push(url);
      if (url.includes("/periods/current")) return response({ code: "PERIOD_NOT_FOUND", message: "No period" }, 404);
      if (url.includes("/summary")) return response({ summary });
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const { result } = renderHook(() => useDashboardData(period, true));
    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));
    act(() => result.current.refresh());
    await waitFor(() => expect(result.current.periodStatus).toBe("not-found"));
    expect(requestedUrls.some((url) => url.includes("/defaults"))).toBe(false);
  });

  it("retries selected suggestions and rejects an older selection response", async () => {
    let suggestionRequestCount = 0;
    let resolveFirst!: (value: Response) => void;
    let resolveSecond!: (value: Response) => void;
    installBaseApi((url) => {
      if (url.includes("/expenses/suggestions")) {
        suggestionRequestCount += 1;
        return new Promise<Response>((resolve) => {
          if (suggestionRequestCount === 1) resolveFirst = resolve;
          else resolveSecond = resolve;
        });
      }
      if (url.includes("/summary")) return response({ summary });
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const { result } = renderHook(() => useDashboardData(period));
    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));

    act(() => result.current.selectBreakdown("repeated-expenses"));
    await waitFor(() => expect(suggestionRequestCount).toBe(1));
    act(() => {
      result.current.selectBreakdown("tag-spending");
      result.current.selectBreakdown("repeated-expenses");
    });
    await waitFor(() => expect(suggestionRequestCount).toBe(2));

    await act(async () => {
      resolveFirst(response({ data: [{ name: "Old", recencyBucket: "today" }], hasMore: false }));
      resolveSecond(response({ data: [{ name: "New", recencyBucket: "today" }], hasMore: false }));
    });

    await waitFor(() => {
      expect(result.current.sections.suggestions.status).toBe("success");
    });
    if (result.current.sections.suggestions.status === "success") {
      expect(result.current.sections.suggestions.suggestions[0].name).toBe("New");
    }
  });

  it("keeps a failed section explicit while unrelated sections complete", async () => {
    installBaseApi((url) => {
      if (url.includes("/summary")) return response({ code: "INTERNAL_SERVER_ERROR", message: "Summary failed" }, 500);
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const { result } = renderHook(() => useDashboardData(period));

    await waitFor(() => expect(result.current.sections.summary.status).toBe("error"));
    expect(result.current.sections.recentExpenses.status).toBe("empty");
    expect(result.current.sections.healthScore.status).toBe("success");
    expect(result.current.data.summary).toBeNull();
  });

  it("verifies period metadata before retrying summary without reloading completed sections", async () => {
    let summaryRequestCount = 0;
    const requestedUrls: string[] = [];
    installBaseApi((url) => {
      requestedUrls.push(url);
      if (url.includes("/periods/current")) return response({ period });
      if (url.includes("/summary")) {
        summaryRequestCount += 1;
        return summaryRequestCount === 1
          ? response({ code: "INTERNAL_SERVER_ERROR", message: "Summary failed" }, 500)
          : response({ summary });
      }
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const { result } = renderHook(() => useDashboardData(period));
    await waitFor(() => expect(result.current.sections.summary.status).toBe("error"));
    await waitFor(() => {
      expect(result.current.sections.recentExpenses.status).toBe("empty");
      expect(result.current.sections.healthScore.status).toBe("success");
    });

    const completedSectionRequests = requestedUrls.filter(
      (url) => url.includes("/expenses?") || url.includes("/health-score?"),
    );
    act(() => result.current.retry("summary"));

    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));
    expect(requestedUrls.filter((url) => url.includes("/periods/current")).length).toBe(1);
    expect(requestedUrls.filter((url) => url.includes("/expenses?")).length).toBe(
      completedSectionRequests.filter((url) => url.includes("/expenses?")).length,
    );
    expect(requestedUrls.filter((url) => url.includes("/health-score?")).length).toBe(
      completedSectionRequests.filter((url) => url.includes("/health-score?")).length,
    );
  });

  it("ignores a late response from the previous period", async () => {
    let resolveFirstSummary!: (value: Response) => void;
    let resolveSecondSummary!: (value: Response) => void;
    installBaseApi((url) => {
      if (url.includes("/summary?year=2026&month=5")) return new Promise<Response>((resolve) => { resolveFirstSummary = resolve; });
      if (url.includes("/summary?year=2026&month=6")) return new Promise<Response>((resolve) => { resolveSecondSummary = resolve; });
      if (url.includes("/health-score?")) return new Promise<Response>(() => {});
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const { result, rerender } = renderHook(({ selectedPeriod }) => useDashboardData(selectedPeriod), {
      initialProps: { selectedPeriod: period },
    });
    const nextPeriod = buildPeriod({ ...period, id: "period-2", month: 6 });
    rerender({ selectedPeriod: nextPeriod });

    await act(async () => {
      resolveFirstSummary(response({ summary }));
      resolveSecondSummary(response({ summary: { ...summary, periodId: nextPeriod.id, month: 6 } }));
    });

    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));
    if (result.current.sections.summary.status === "success") {
      expect(result.current.sections.summary.data.periodId).toBe(nextPeriod.id);
    }
  });
});
