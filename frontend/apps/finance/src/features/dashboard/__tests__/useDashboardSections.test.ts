import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildPeriod, buildPeriodSummary } from "@gofin/test-utils";
import { useDashboardSections } from "../hooks/useDashboardSections";

const period = buildPeriod({ id: "period-1", year: 2026, month: 5 });
const summary = buildPeriodSummary({ periodId: period.id, year: period.year, month: period.month });

function response(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function installMobileViewport(): void {
  window.matchMedia = vi.fn().mockImplementation(() => ({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
}

function installBaseApi(onRequest: (url: string, init?: RequestInit) => Response): void {
  globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) =>
    Promise.resolve(onRequest(String(input), init))) as typeof fetch;
}

describe("useDashboardSections", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    installMobileViewport();
  });

  it("loads only base sections on mobile and preserves scoped force refresh", async () => {
    const requestedUrls: string[] = [];
    const cacheControls: string[] = [];
    installBaseApi((url, init) => {
      requestedUrls.push(url);
      cacheControls.push(new Headers(init?.headers).get("Cache-Control") ?? "");
      if (url.includes("/summary")) return response({ summary });
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const generationRef = { current: 0 };
    const periodRef = { current: period };
    const { result } = renderHook(() => useDashboardSections(generationRef, periodRef));

    act(() => result.current.startPeriodSections());
    await waitFor(() => expect(result.current.sections.summary.status).toBe("success"));

    expect(requestedUrls.some((url) => url.includes("spending/by-tag"))).toBe(false);
    expect(requestedUrls.some((url) => url.includes("spending/trends"))).toBe(false);

    act(() => {
      result.current.clearRequestedSection("recentExpenses");
      void result.current.loadSection("recentExpenses", true);
    });
    await waitFor(() => expect(cacheControls).toContain("no-cache"));
  });

  it("loads suggestions only after repeated expenses is selected on desktop", async () => {
    window.matchMedia = vi.fn().mockImplementation(() => ({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }));
    installBaseApi((url) => {
      if (url.includes("/expenses/suggestions")) return response({ data: [], total: 0, page: 1, pageSize: 10, hasMore: false });
      if (url.includes("/summary")) return response({ summary });
      if (url.includes("/health-score?")) return response({ healthScore: { configureBudget: true } });
      if (url.includes("/health-score/trend")) return response({ trends: [] });
      if (url.includes("/expenses?")) return response({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false });
      if (url.includes("/prorata/upcoming")) return response({ schedules: [] });
      return response({});
    });

    const generationRef = { current: 0 };
    const periodRef = { current: period };
    const { result } = renderHook(() => useDashboardSections(generationRef, periodRef));

    act(() => result.current.selectBreakdown("repeated-expenses"));
    await waitFor(() => expect(result.current.sections.suggestions.status).toBe("empty"));
    expect(result.current.breakdownChart).toBe("repeated-expenses");
  });
});
