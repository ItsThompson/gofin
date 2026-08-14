import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useHistoryData } from "../hooks/useHistoryData";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockPeriods = [
  {
    id: "p1",
    userId: "user-1",
    year: 2026,
    month: 3,
    budgetAmount: 300000,
    reportingCurrency: "USD",
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: "2026-03-01T00:00:00Z",
    updatedAt: "2026-03-01T00:00:00Z",
  },
  {
    id: "p2",
    userId: "user-1",
    year: 2026,
    month: 2,
    budgetAmount: 250000,
    reportingCurrency: "USD",
    essentialsPercent: 50,
    desiresPercent: 30,
    savingsPercent: 20,
    createdAt: "2026-02-01T00:00:00Z",
    updatedAt: "2026-02-01T00:00:00Z",
  },
];

function mockSummaryResponse(totalSpent: number) {
  return {
    ok: true,
    status: 200,
    json: () =>
      Promise.resolve({
        summary: {
          periodId: "p",
          totalBudget: 300000,
          totalSpent,
          remaining: 300000 - totalSpent,
          daysInPeriod: 31,
          daysElapsed: 31,
          isOnTrack: true,
          essentials: { allocated: 150000, spent: 0, remaining: 150000, percentUsed: 0 },
          desires: { allocated: 90000, spent: 0, remaining: 90000, percentUsed: 0 },
          savings: { allocated: 60000, spent: 0, remaining: 60000, percentUsed: 0 },
        },
      }),
  };
}

describe("useHistoryData", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("starts in loading state", () => {
    mockFetch.mockReturnValueOnce(new Promise(() => {}));
    const { result } = renderHook(() => useHistoryData());

    expect(result.current.loading).toBe(true);
    expect(result.current.periods).toEqual([]);
  });

  it("fetches periods and computes totalSpent/surplus", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: mockPeriods }),
    });
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(200000)); // March
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(280000)); // Feb

    const { result } = renderHook(() => useHistoryData());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.periods).toHaveLength(2);
    expect(result.current.periods[0]).toEqual({
      period: mockPeriods[0],
      status: "loaded",
      totalSpent: 200000,
      surplus: 100000, // 300000 - 200000
    });
    expect(result.current.periods[1]).toEqual({
      period: mockPeriods[1],
      status: "loaded",
      totalSpent: 280000,
      surplus: -30000, // 250000 - 280000
    });
  });

  it("marks a period whose summary fetch fails as unavailable", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: [mockPeriods[0]] }),
    });
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const { result } = renderHook(() => useHistoryData());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.periods).toEqual([
      { period: mockPeriods[0], status: "unavailable" },
    ]);
  });

  it("keeps surviving periods loaded when one summary fetch fails", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: mockPeriods }),
    });
    mockFetch.mockRejectedValueOnce(new Error("Network error")); // March
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(280000)); // Feb

    const { result } = renderHook(() => useHistoryData());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.periods).toEqual([
      { period: mockPeriods[0], status: "unavailable" },
      {
        period: mockPeriods[1],
        status: "loaded",
        totalSpent: 280000,
        surplus: -30000,
      },
    ]);
  });

  it("marks every period unavailable when all summary fetches fail", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: mockPeriods }),
    });
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = renderHook(() => useHistoryData());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.periods).toEqual([
      { period: mockPeriods[0], status: "unavailable" },
      { period: mockPeriods[1], status: "unavailable" },
    ]);
  });

  it("loads a genuine zero spend as a loaded row", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: [mockPeriods[0]] }),
    });
    mockFetch.mockResolvedValueOnce(mockSummaryResponse(0));

    const { result } = renderHook(() => useHistoryData());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.periods).toEqual([
      {
        period: mockPeriods[0],
        status: "loaded",
        totalSpent: 0,
        surplus: 300000,
      },
    ]);
  });

  it("returns empty periods when fetch fails entirely", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () =>
        Promise.resolve({ code: "INTERNAL_SERVER_ERROR", message: "Server error" }),
    });

    const { result } = renderHook(() => useHistoryData());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.periods).toEqual([]);
  });

  it("returns empty periods when no periods exist", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ periods: [] }),
    });

    const { result } = renderHook(() => useHistoryData());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.periods).toEqual([]);
  });
});
