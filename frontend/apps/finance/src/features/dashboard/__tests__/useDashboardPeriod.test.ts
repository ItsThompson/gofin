import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildPeriod } from "@gofin/test-utils";
import { useDashboardPeriod } from "../hooks/useDashboardPeriod";

const period = buildPeriod({ id: "period-1", year: 2026, month: 5 });

function response(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

describe("useDashboardPeriod", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("verifies the current period on refresh and advances the data version", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ period }));
    globalThis.fetch = fetchMock as typeof fetch;
    const { result } = renderHook(() => useDashboardPeriod(period));
    const initialVersion = result.current.refreshVersion;

    act(() => result.current.refresh());

    await waitFor(() => expect(result.current.status).toBe("active"));
    expect(result.current.refreshVersion).toBeGreaterThan(initialVersion);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/finance/periods/current"),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("enters historical recovery without requesting defaults", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ code: "PERIOD_NOT_FOUND", message: "Missing" }, 404));
    globalThis.fetch = fetchMock as typeof fetch;
    const { result } = renderHook(() => useDashboardPeriod(period, true));

    act(() => result.current.refresh());

    await waitFor(() => expect(result.current.status).toBe("not-found"));
    expect(result.current.recovery).toBeNull();
    expect(String(fetchMock.mock.calls[0]?.[0])).toContain("/api/finance/periods/current");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
