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
