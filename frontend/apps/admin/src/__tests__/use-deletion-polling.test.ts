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
  };

  beforeEach(() => {
    vi.useFakeTimers();
    mockFetch.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does not poll when enabled is false", async () => {
    renderHook(() => useDeletionPolling({ ...defaultOptions, enabled: false }));

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("does not poll when jobId is empty", async () => {
    renderHook(() => useDeletionPolling({ ...defaultOptions, jobId: "" }));

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("polls at the specified interval when enabled", async () => {
    mockFetch.mockResolvedValue(buildPollResponse("running"));

    renderHook(() => useDeletionPolling(defaultOptions));

    // No call immediately
    expect(mockFetch).not.toHaveBeenCalled();

    // First tick
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledWith(
      "/api/datarights/deletions/job-1",
      expect.objectContaining({ credentials: "include" }),
    );

    // Second tick
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it("calls onStatusChange with status on each poll", async () => {
    const onStatusChange = vi.fn();
    mockFetch.mockResolvedValue(buildPollResponse("running"));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onStatusChange }));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onStatusChange).toHaveBeenCalledWith("running", undefined);
  });

  it("stops polling and calls onCompleted when status is completed", async () => {
    const onCompleted = vi.fn();
    const onStatusChange = vi.fn();
    mockFetch.mockResolvedValue(buildPollResponse("completed"));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onCompleted, onStatusChange }));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onCompleted).toHaveBeenCalledTimes(1);
    expect(onStatusChange).toHaveBeenCalledWith("completed", undefined);

    // No further polls after terminal state
    mockFetch.mockClear();
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("stops polling and calls onFailed when status is failed", async () => {
    const onFailed = vi.fn();
    const onStatusChange = vi.fn();
    mockFetch.mockResolvedValue(buildPollResponse("failed", "auth provider timeout"));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onFailed, onStatusChange }));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onFailed).toHaveBeenCalledWith("auth provider timeout");
    expect(onStatusChange).toHaveBeenCalledWith("failed", "auth provider timeout");

    // No further polls after terminal state
    mockFetch.mockClear();
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("cleans up interval on unmount", async () => {
    mockFetch.mockResolvedValue(buildPollResponse("running"));

    const { unmount } = renderHook(() => useDeletionPolling(defaultOptions));

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);

    unmount();
    mockFetch.mockClear();

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("continues polling on network error (resilient to transient failures)", async () => {
    const onFailed = vi.fn();
    const onStatusChange = vi.fn();

    // First poll: network error
    mockFetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    // Second poll: success
    mockFetch.mockResolvedValueOnce(buildPollResponse("running"));

    renderHook(() => useDeletionPolling({ ...defaultOptions, onFailed, onStatusChange }));

    // First tick: network error, should NOT call onFailed
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onFailed).not.toHaveBeenCalled();
    expect(onStatusChange).not.toHaveBeenCalled();

    // Second tick: success
    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(onStatusChange).toHaveBeenCalledWith("running", undefined);
    expect(onFailed).not.toHaveBeenCalled();
  });

  it("stops polling when enabled changes from true to false", async () => {
    mockFetch.mockResolvedValue(buildPollResponse("running"));

    const { rerender } = renderHook(
      (props: UseDeletionPollingOptions) => useDeletionPolling(props),
      { initialProps: defaultOptions },
    );

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);

    // Disable polling
    rerender({ ...defaultOptions, enabled: false });
    mockFetch.mockClear();

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("uses default interval of 2500ms when intervalMs not specified", async () => {
    mockFetch.mockResolvedValue(buildPollResponse("running"));
    const optionsWithoutInterval = { ...defaultOptions };
    delete (optionsWithoutInterval as Partial<UseDeletionPollingOptions>).intervalMs;

    renderHook(() => useDeletionPolling(optionsWithoutInterval));

    // At 2400ms, should not have polled yet
    await act(async () => {
      vi.advanceTimersByTime(2400);
    });
    expect(mockFetch).not.toHaveBeenCalled();

    // At 2500ms, should have polled
    await act(async () => {
      vi.advanceTimersByTime(100);
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});
