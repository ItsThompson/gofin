import { describe, it, expect } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { createMockApi, mockSequence } from "@gofin/test-utils";
import { useExpenseFrecencyData } from "../hooks/useExpenseFrecencyData";

const suggestions = [
  {
    name: "Groceries",
    amount: 50000,
    currency: "USD",
    expenseType: "essentials" as const,
    tagId: "tag-food",
    frequency: 114,
    lastUsedAt: "2026-05-02T10:00:00Z",
    recencyBucket: "last_7_days" as const,
    frecencyScore: 145,
  },
  {
    name: "Coffee",
    amount: 4500,
    currency: "USD",
    expenseType: "desires" as const,
    tagId: "tag-social",
    frequency: 42,
    lastUsedAt: "2026-05-01T09:00:00Z",
    recencyBucket: "today" as const,
    frecencyScore: 90,
  },
];

const olderSuggestion = {
  name: "Old Bus Fare",
  amount: 350,
  currency: "USD",
  expenseType: "essentials" as const,
  tagId: "tag-transit",
  frequency: 20,
  lastUsedAt: "2026-01-01T09:00:00Z",
  recencyBucket: "older" as const,
  frecencyScore: 20,
};

describe("useExpenseFrecencyData", () => {
  it("fetches page 1 suggestions and exposes success state", async () => {
    const mockApi = createMockApi({
      "/api/expenses/suggestions": {
        body: { data: suggestions, total: 2, page: 1, pageSize: 2, hasMore: false },
      },
    });
    globalThis.fetch = mockApi as unknown as typeof fetch;

    const { result } = renderHook(() => useExpenseFrecencyData({ pageSize: 2 }));

    expect(result.current.status).toBe("loading");
    await waitFor(() => {
      expect(result.current.status).toBe("success");
    });

    expect(result.current.suggestions).toEqual(suggestions);
    expect(result.current.errorMessage).toBeNull();
    expect(mockApi._calls[0].url).toContain("/api/expenses/suggestions?page=1&pageSize=2");
  });

  it("fetches later pages when older suggestions underfill the chart", async () => {
    const mockApi = createMockApi({
      "/api/expenses/suggestions": mockSequence([
        {
          body: {
            data: [olderSuggestion],
            total: 3,
            page: 1,
            pageSize: 2,
            hasMore: true,
          },
        },
        {
          body: {
            data: suggestions,
            total: 3,
            page: 2,
            pageSize: 2,
            hasMore: false,
          },
        },
      ]),
    });
    globalThis.fetch = mockApi as unknown as typeof fetch;

    const { result } = renderHook(() => useExpenseFrecencyData({ pageSize: 2 }));

    await waitFor(() => {
      expect(result.current.status).toBe("success");
    });

    expect(result.current.suggestions).toEqual(suggestions);
    expect(mockApi._calls.map((call) => call.url)).toEqual([
      "/api/expenses/suggestions?page=1&pageSize=2",
      "/api/expenses/suggestions?page=2&pageSize=2",
    ]);
  });

  it("ignores older suggestions", async () => {
    globalThis.fetch = createMockApi({
      "/api/expenses/suggestions": {
        body: {
          data: [olderSuggestion, ...suggestions],
          total: 3,
          page: 1,
          pageSize: 10,
          hasMore: false,
        },
      },
    }) as unknown as typeof fetch;

    const { result } = renderHook(() => useExpenseFrecencyData());

    await waitFor(() => {
      expect(result.current.status).toBe("success");
    });

    expect(result.current.suggestions).toEqual(suggestions);
  });

  it("exposes empty state when no current suggestions are returned", async () => {
    globalThis.fetch = createMockApi({
      "/api/expenses/suggestions": {
        body: { data: [olderSuggestion], total: 1, page: 1, pageSize: 10, hasMore: false },
      },
    }) as unknown as typeof fetch;

    const { result } = renderHook(() => useExpenseFrecencyData());

    await waitFor(() => {
      expect(result.current.status).toBe("empty");
    });

    expect(result.current.suggestions).toEqual([]);
    expect(result.current.errorMessage).toBeNull();
  });

  it("exposes error state when the request fails", async () => {
    globalThis.fetch = createMockApi({
      "/api/expenses/suggestions": {
        status: 500,
        body: { code: "INTERNAL_SERVER_ERROR", message: "Suggestions failed" },
      },
    }) as unknown as typeof fetch;

    const { result } = renderHook(() => useExpenseFrecencyData());

    await waitFor(() => {
      expect(result.current.status).toBe("error");
    });

    expect(result.current.suggestions).toEqual([]);
    expect(result.current.errorMessage).toBe("Suggestions failed");
  });

  it("aborts the in-flight request on unmount", () => {
    let capturedSignal: AbortSignal | undefined;
    globalThis.fetch = ((_input: RequestInfo | URL, init?: RequestInit) => {
      capturedSignal = init?.signal ?? undefined;
      return new Promise(() => {});
    }) as unknown as typeof fetch;

    const { unmount } = renderHook(() => useExpenseFrecencyData());
    unmount();

    expect(capturedSignal?.aborted).toBe(true);
  });
});
