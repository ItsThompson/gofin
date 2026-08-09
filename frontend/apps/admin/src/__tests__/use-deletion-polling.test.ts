import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDeletionPolling } from "@/hooks/useDeletionPolling";
import type { UseDeletionPollingOptions } from "@/hooks/useDeletionPolling";

const mockFetch = vi.fn();
global.fetch = mockFetch;

function buildPollResponse(status: string, error: string | null = null) {
  return {
    ok: true,
    status: 200,
    json: () => Promise.resolve({
      id: "job-1",
      userId: "user-1",
      status,
      error,
      createdAt: "2026-05-10T00:00:00Z",
      completedAt: status === "completed" ? "2026-05-10T00:01:00Z" : null,
    }),
  };
}

describe("useDeletionPolling", () => {
  const defaultOptions: UseDeletionPollingOptions = {
    jobId: "job-1",
    enabled: true,
    intervalMs: 2500,
    onStatusChange: vi.fn(),
    onCompleted: vi.fn(),
    onFailed: vi.fn(),
    onStatusUnavailable: vi.fn(),
  };

  beforeEach(() => {
    vi.useFakeTimers();
    mockFetch.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does not poll when jobId is empty", async () => {
    renderHook(() => useDeletionPolling({ ...defaultOptions, jobId: "" }));

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("fetches the correct deletion endpoint", async () => {
    mockFetch.mockResolvedValue(buildPollResponse("running"));

    renderHook(() => useDeletionPolling(defaultOptions));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/datarights/deletions/job-1",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("calls onStatusChange with status and error on each poll", async () => {
    const onStatusChange = vi.fn();
    mockFetch.mockResolvedValue(buildPollResponse("running"));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onStatusChange }));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onStatusChange).toHaveBeenCalledWith("running", undefined);
  });

  it("calls onCompleted and onStatusChange when status is completed", async () => {
    const onCompleted = vi.fn();
    const onStatusChange = vi.fn();
    mockFetch.mockResolvedValue(buildPollResponse("completed"));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onCompleted, onStatusChange }));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onCompleted).toHaveBeenCalledTimes(1);
    expect(onStatusChange).toHaveBeenCalledWith("completed", undefined);
  });

  it("calls onFailed with error message when status is failed", async () => {
    const onFailed = vi.fn();
    const onStatusChange = vi.fn();
    mockFetch.mockResolvedValue(buildPollResponse("failed", "auth provider timeout"));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onFailed, onStatusChange }));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onFailed).toHaveBeenCalledWith("auth provider timeout");
    expect(onStatusChange).toHaveBeenCalledWith("failed", "auth provider timeout");
  });

  it("calls onFailed with 'Unknown error' when error field is null", async () => {
    const onFailed = vi.fn();
    mockFetch.mockResolvedValue(buildPollResponse("failed", null));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onFailed }));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onFailed).toHaveBeenCalledWith("Unknown error");
  });

  it("stops polling after terminal state is reached", async () => {
    mockFetch.mockResolvedValue(buildPollResponse("completed"));

    renderHook(() => useDeletionPolling(defaultOptions));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    mockFetch.mockClear();

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("does not call onFailed on network errors (resilient polling)", async () => {
    const onFailed = vi.fn();
    const onStatusChange = vi.fn();

    mockFetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    mockFetch.mockResolvedValueOnce(buildPollResponse("running"));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onFailed, onStatusChange }));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onFailed).not.toHaveBeenCalled();
    expect(onStatusChange).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onStatusChange).toHaveBeenCalledWith("running", undefined);
  });

  it("calls onStatusUnavailable and stops once the status endpoint keeps failing", async () => {
    const onStatusUnavailable = vi.fn();
    const onFailed = vi.fn();
    mockFetch.mockRejectedValue(new TypeError("Failed to fetch"));

    renderHook(() =>
      useDeletionPolling({ ...defaultOptions, onFailed, onStatusUnavailable }),
    );

    await act(async () => {
      vi.advanceTimersByTime(7500);
    });

    expect(onStatusUnavailable).toHaveBeenCalledTimes(1);
    // A poll that gave up is not a failed deletion.
    expect(onFailed).not.toHaveBeenCalled();

    mockFetch.mockClear();
    await act(async () => {
      vi.advanceTimersByTime(10000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
    expect(onStatusUnavailable).toHaveBeenCalledTimes(1);
  });
});
