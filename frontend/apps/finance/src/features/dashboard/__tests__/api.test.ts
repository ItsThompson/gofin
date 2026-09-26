import { beforeEach, describe, expect, it, vi } from "vitest";
import { dashboardApi } from "../api";
import { expenseSuggestionsApi } from "../../expense-autocomplete/api";

function mockSuccess(body: unknown) {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: () => Promise.resolve(body),
  }));
}

function expectRefreshRequest(fetchMock: ReturnType<typeof vi.fn>, url: string, callIndex: number) {
  expect(fetchMock).toHaveBeenNthCalledWith(
    callIndex,
    url,
    expect.objectContaining({
      headers: {
        "Content-Type": "application/json",
        "Cache-Control": "no-cache",
      },
    }),
  );
}

describe("dashboard transport options", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("forwards force refresh to a dashboard read without changing its URL", async () => {
    mockSuccess({ summary: null });

    await dashboardApi.getSummary(2026, 5, { forceRefresh: true });

    expect(fetch).toHaveBeenCalledWith(
      "/api/finance/summary?year=2026&month=5",
      expect.objectContaining({
        headers: {
          "Content-Type": "application/json",
          "Cache-Control": "no-cache",
        },
      }),
    );
  });

  it("forwards force refresh to every targeted dashboard read", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    });
    vi.stubGlobal("fetch", fetchMock);

    await dashboardApi.getCurrentPeriod(2026, 5, { forceRefresh: true });
    await dashboardApi.getSummary(2026, 5, { forceRefresh: true });
    await dashboardApi.getHealthScore(2026, 5, { forceRefresh: true });
    await dashboardApi.getHealthScoreTrend(2026, 5, 6, { forceRefresh: true });
    await dashboardApi.getTagSpending(2026, 5, { forceRefresh: true });
    await dashboardApi.getCumulative(2026, 5, { forceRefresh: true });
    await dashboardApi.getComparison(2026, 5, { forceRefresh: true });
    await dashboardApi.getUpcomingProRata({ forceRefresh: true });
    await dashboardApi.getRecentExpenses(2026, 5, 5, { forceRefresh: true });
    await dashboardApi.getTrend(2026, 5, 6, { forceRefresh: true });

    const urls = [
      "/api/finance/periods/current?year=2026&month=5",
      "/api/finance/summary?year=2026&month=5",
      "/api/finance/health-score?year=2026&month=5",
      "/api/finance/health-score/trend?year=2026&month=5&months=6",
      "/api/finance/spending/by-tag?year=2026&month=5",
      "/api/finance/spending/cumulative?year=2026&month=5",
      "/api/finance/spending/comparison?year=2026&month=5",
      "/api/finance/prorata/upcoming",
      "/api/expenses?year=2026&month=5&page=1&pageSize=5",
      "/api/finance/spending/trends?year=2026&month=5&months=6",
    ];

    for (const [index, url] of urls.entries()) {
      expectRefreshRequest(fetchMock, url, index + 1);
    }
  });

  it("forwards force refresh to selected suggestions", async () => {
    mockSuccess({ data: [], total: 0, page: 1, pageSize: 50, hasMore: false });

    await expenseSuggestionsApi.getSuggestions(1, 50, { forceRefresh: true });

    expect(fetch).toHaveBeenCalledWith(
      "/api/expenses/suggestions?page=1&pageSize=50",
      expect.objectContaining({
        headers: {
          "Content-Type": "application/json",
          "Cache-Control": "no-cache",
        },
      }),
    );
  });
});
