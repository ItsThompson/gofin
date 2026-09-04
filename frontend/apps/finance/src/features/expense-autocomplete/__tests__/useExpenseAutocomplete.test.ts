import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useExpenseAutocomplete } from "../hooks/useExpenseAutocomplete";
import type { ExpenseSuggestion, ExpenseSuggestionsResponse } from "../types";

const mockFetch = vi.fn();
global.fetch = mockFetch;

function buildSuggestion(overrides: Partial<ExpenseSuggestion> = {}): ExpenseSuggestion {
  return {
    name: "Groceries",
    originalTransactionAmountInMinorUnits: 7423,
    transactionCurrencyCode: "USD",
    expenseType: "essentials",
    tagId: "tag-groceries",
    frequency: 12,
    lastUsedAt: "2026-05-28T19:02:11Z",
    recencyBucket: "last_7_days",
    frecencyScore: 48,
    ...overrides,
  };
}

function buildResponse(
  data: ExpenseSuggestion[],
  overrides: Partial<ExpenseSuggestionsResponse> = {},
): ExpenseSuggestionsResponse {
  return {
    data,
    total: data.length,
    page: 1,
    pageSize: 50,
    hasMore: false,
    ...overrides,
  };
}

function mockApiResponse(response: ExpenseSuggestionsResponse) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve(response),
  });
}

describe("useExpenseAutocomplete", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("loads the first suggestion page on mount", async () => {
    const groceries = buildSuggestion();
    mockApiResponse(buildResponse([groceries], { hasMore: true }));

    const { result } = renderHook(() => useExpenseAutocomplete());

    expect(result.current.state.isInitialLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.state.isInitialLoading).toBe(false);
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/expenses/suggestions?page=1&pageSize=50",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(result.current.state.candidates).toEqual([groceries]);
    expect(result.current.state.page).toBe(1);
    expect(result.current.state.hasMore).toBe(true);
    expect(result.current.state.error).toBeNull();
  });

  it("fuzzy matches loaded candidates without changing the candidate cache", async () => {
    const groceries = buildSuggestion({ name: "Groceries" });
    const coffee = buildSuggestion({ name: "Coffee", frecencyScore: 20 });
    mockApiResponse(buildResponse([groceries, coffee]));

    const { result } = renderHook(() => useExpenseAutocomplete());

    await waitFor(() => {
      expect(result.current.state.candidates).toHaveLength(2);
    });

    act(() => {
      result.current.actions.setQuery("grc");
    });

    expect(result.current.state.visibleSuggestions).toEqual([groceries]);
    expect(result.current.state.candidates).toEqual([groceries, coffee]);
  });

  it("returns an empty visible list when the query has no matches", async () => {
    mockApiResponse(buildResponse([buildSuggestion({ name: "Groceries" })]));

    const { result } = renderHook(() => useExpenseAutocomplete());

    await waitFor(() => {
      expect(result.current.state.isInitialLoading).toBe(false);
    });

    act(() => {
      result.current.actions.setQuery("zzz");
    });

    expect(result.current.state.visibleSuggestions).toEqual([]);
  });

  it("limits visible suggestions to five matches", async () => {
    const suggestions = Array.from({ length: 7 }, (_, index) =>
      buildSuggestion({ name: `Coffee ${index + 1}`, frecencyScore: 100 - index }),
    );
    mockApiResponse(buildResponse(suggestions));

    const { result } = renderHook(() => useExpenseAutocomplete());

    await waitFor(() => {
      expect(result.current.state.candidates).toHaveLength(7);
    });

    act(() => {
      result.current.actions.setQuery("coffee");
    });

    expect(result.current.state.visibleSuggestions).toHaveLength(5);
    expect(result.current.state.visibleSuggestions.map((suggestion) => suggestion.name)).toEqual([
      "Coffee 1",
      "Coffee 2",
      "Coffee 3",
      "Coffee 4",
      "Coffee 5",
    ]);
  });

  it("keeps initial failures non-blocking", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      json: () => Promise.resolve({ code: "internal_server_error", message: "failed" }),
    });

    const { result } = renderHook(() => useExpenseAutocomplete());

    await waitFor(() => {
      expect(result.current.state.isInitialLoading).toBe(false);
    });

    expect(result.current.state.candidates).toEqual([]);
    expect(result.current.state.visibleSuggestions).toEqual([]);
    expect(result.current.state.error).toBe("Suggestions are unavailable right now.");
  });

  it("dedupes candidates by exact name and keeps the first ranked record", async () => {
    const firstGroceries = buildSuggestion({ name: "Groceries", originalTransactionAmountInMinorUnits: 1000, frecencyScore: 50 });
    const duplicateGroceries = buildSuggestion({ name: "Groceries", originalTransactionAmountInMinorUnits: 2000, frecencyScore: 10 });
    const coffee = buildSuggestion({ name: "Coffee", originalTransactionAmountInMinorUnits: 500, frecencyScore: 20 });
    mockApiResponse(buildResponse([firstGroceries, duplicateGroceries, coffee]));

    const { result } = renderHook(() => useExpenseAutocomplete());

    await waitFor(() => {
      expect(result.current.state.isInitialLoading).toBe(false);
    });

    expect(result.current.state.candidates).toEqual([firstGroceries, coffee]);
  });

  it("loads the next page and appends deduped candidates", async () => {
    const groceries = buildSuggestion({ name: "Groceries", originalTransactionAmountInMinorUnits: 1000, frecencyScore: 50 });
    const duplicateGroceries = buildSuggestion({ name: "Groceries", originalTransactionAmountInMinorUnits: 2000, frecencyScore: 10 });
    const coffee = buildSuggestion({ name: "Coffee", originalTransactionAmountInMinorUnits: 500, frecencyScore: 20 });
    mockApiResponse(buildResponse([groceries], { hasMore: true }));
    mockApiResponse(buildResponse([duplicateGroceries, coffee], { page: 2, hasMore: false }));

    const { result } = renderHook(() => useExpenseAutocomplete());

    await waitFor(() => {
      expect(result.current.state.isInitialLoading).toBe(false);
    });

    await act(async () => {
      await result.current.actions.loadMore();
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/expenses/suggestions?page=2&pageSize=50",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(result.current.state.candidates).toEqual([groceries, coffee]);
    expect(result.current.state.page).toBe(2);
    expect(result.current.state.hasMore).toBe(false);
    expect(result.current.state.error).toBeNull();
  });

  it("keeps loaded candidates usable when loadMore fails", async () => {
    const groceries = buildSuggestion({ name: "Groceries" });
    mockApiResponse(buildResponse([groceries], { hasMore: true }));
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      json: () => Promise.resolve({ code: "internal_server_error", message: "failed" }),
    });

    const { result } = renderHook(() => useExpenseAutocomplete());

    await waitFor(() => {
      expect(result.current.state.isInitialLoading).toBe(false);
    });

    await act(async () => {
      await result.current.actions.loadMore();
    });

    expect(result.current.state.candidates).toEqual([groceries]);
    expect(result.current.state.hasMore).toBe(true);
    expect(result.current.state.error).toBe("Suggestions are unavailable right now.");
  });

  it("does not request another page when hasMore is false", async () => {
    mockApiResponse(buildResponse([buildSuggestion({ name: "Groceries" })], { hasMore: false }));

    const { result } = renderHook(() => useExpenseAutocomplete());

    await waitFor(() => {
      expect(result.current.state.isInitialLoading).toBe(false);
    });

    await act(async () => {
      await result.current.actions.loadMore();
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("aborts the initial request on unmount so stale responses are ignored", () => {
    let requestSignal: AbortSignal | undefined;
    mockFetch.mockImplementationOnce((_: string, options: RequestInit) => {
      requestSignal = options.signal as AbortSignal;
      return new Promise(() => {});
    });

    const { unmount } = renderHook(() => useExpenseAutocomplete());

    unmount();

    expect(requestSignal?.aborted).toBe(true);
  });
});
