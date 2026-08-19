import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { jsonResponse, mockPeriod } from "../__mocks__";
import { usePeriodContext } from "../hooks/usePeriodContext";

const mockFetch = vi.fn();
global.fetch = mockFetch;

describe("usePeriodContext", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("starts loading and resolves to active with the fetched period", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ period: mockPeriod }));

    const { result } = renderHook(() => usePeriodContext(2026, 5));

    expect(result.current).toEqual({ status: "loading" });

    await waitFor(() => {
      expect(result.current).toEqual({ status: "active", period: mockPeriod });
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/finance/periods/current?year=2026&month=5",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("resolves to missing when the API reports PERIOD_NOT_FOUND", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse(
        { code: "PERIOD_NOT_FOUND", message: "No period for 2026-05" },
        404,
      ),
    );

    const { result } = renderHook(() => usePeriodContext(2026, 5));

    await waitFor(() => {
      expect(result.current).toEqual({ status: "missing" });
    });
  });

  it("resolves to error with a message on unexpected failure", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ code: "INTERNAL_ERROR", message: "boom" }, 500),
    );

    const { result } = renderHook(() => usePeriodContext(2026, 5));

    await waitFor(() => {
      expect(result.current).toEqual({
        status: "error",
        message: "Failed to load budget period context.",
      });
    });
  });

  it("refetches when the year or month changes", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ period: mockPeriod }));

    const { result, rerender } = renderHook(
      ({ year, month }: { year: number; month: number }) =>
        usePeriodContext(year, month),
      { initialProps: { year: 2026, month: 5 } },
    );

    await waitFor(() => {
      expect(result.current).toEqual({ status: "active", period: mockPeriod });
    });

    rerender({ year: 2026, month: 6 });

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/finance/periods/current?year=2026&month=6",
        expect.objectContaining({ credentials: "include" }),
      );
    });
  });
});
