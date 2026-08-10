import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useUserDeletion } from "@/hooks/useUserDeletion";
import type { DeletionJobResponse } from "@/components/DeleteUserDialog/types";

const mockFetch = vi.fn();
global.fetch = mockFetch;

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

function buildJob(overrides?: Partial<DeletionJobResponse>): DeletionJobResponse {
  return {
    id: "job-1",
    userId: "user-1",
    status: "pending",
    error: null,
    createdAt: "2026-05-10T00:00:00Z",
    completedAt: null,
    ...overrides,
  };
}

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

describe("useUserDeletion", () => {
  const mockOnUserRemoved = vi.fn();

  beforeEach(() => {
    vi.useFakeTimers();
    mockFetch.mockReset();
    mockOnUserRemoved.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts with null deletingUser and empty deletionStates", () => {
    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    expect(result.current.state.deletingUser).toBeNull();
    expect(result.current.state.deletionStates).toEqual({});
    expect(result.current.state.isPolling).toBe(false);
  });

  it("startDeletion sets deletingUser", () => {
    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    act(() => {
      result.current.actions.startDeletion({ id: "user-1", username: "alice" });
    });

    expect(result.current.state.deletingUser).toEqual({ id: "user-1", username: "alice" });
  });

  it("cancelDeletion clears deletingUser", () => {
    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    act(() => {
      result.current.actions.startDeletion({ id: "user-1", username: "alice" });
    });

    act(() => {
      result.current.actions.cancelDeletion();
    });

    expect(result.current.state.deletingUser).toBeNull();
  });

  it("handleDeletionSuccess clears deletingUser, sets pending status, and starts polling", () => {
    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    act(() => {
      result.current.actions.startDeletion({ id: "user-1", username: "alice" });
    });

    act(() => {
      result.current.actions.handleDeletionSuccess(buildJob());
    });

    expect(result.current.state.deletingUser).toBeNull();
    expect(result.current.state.deletionStates["user-1"]).toEqual({
      jobId: "job-1",
      status: "pending",
    });
    expect(result.current.state.isPolling).toBe(true);
  });

  it("calls onUserRemoved and clears state when polling completes", async () => {
    const { toast } = await import("sonner");
    mockFetch.mockResolvedValue(buildPollResponse("completed"));

    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    act(() => {
      result.current.actions.startDeletion({ id: "user-1", username: "alice" });
    });

    act(() => {
      result.current.actions.handleDeletionSuccess(buildJob());
    });

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(mockOnUserRemoved).toHaveBeenCalledWith("user-1");
    expect(result.current.state.isPolling).toBe(false);
    expect(result.current.state.deletionStates["user-1"]).toBeUndefined();
    expect(toast.success).toHaveBeenCalledWith('User "alice" has been deleted');
  });

  it("shows error toast and stops polling when deletion fails", async () => {
    const { toast } = await import("sonner");
    mockFetch.mockResolvedValue(buildPollResponse("failed", "auth provider timeout"));

    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    act(() => {
      result.current.actions.startDeletion({ id: "user-1", username: "alice" });
    });

    act(() => {
      result.current.actions.handleDeletionSuccess(buildJob());
    });

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(mockOnUserRemoved).not.toHaveBeenCalled();
    expect(result.current.state.isPolling).toBe(false);
    expect(result.current.state.deletionStates["user-1"]).toEqual({
      jobId: "job-1",
      status: "failed",
      error: "auth provider timeout",
    });
    expect(toast.error).toHaveBeenCalledWith(
      'Deletion of "alice" failed: auth provider timeout',
    );
  });

  it("warns the operator once when the deletion status becomes unreadable", async () => {
    const { toast } = await import("sonner");
    mockFetch.mockRejectedValue(new TypeError("Failed to fetch"));

    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    act(() => {
      result.current.actions.startDeletion({ id: "user-1", username: "alice" });
    });

    act(() => {
      result.current.actions.handleDeletionSuccess(buildJob());
    });

    await act(async () => {
      vi.advanceTimersByTime(7500);
    });

    expect(toast.error).toHaveBeenCalledWith(
      'Lost contact with the server while deleting "alice". Refresh to check whether it finished.',
    );
    expect(result.current.state.isPolling).toBe(false);
    expect(mockOnUserRemoved).not.toHaveBeenCalled();
    // The last known status stays visible on the row.
    expect(result.current.state.deletionStates["user-1"]).toEqual({
      jobId: "job-1",
      status: "pending",
    });
  });

  it("updates deletionStates on intermediate status changes", async () => {
    mockFetch.mockResolvedValueOnce(buildPollResponse("running"));

    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    act(() => {
      result.current.actions.startDeletion({ id: "user-1", username: "alice" });
    });

    act(() => {
      result.current.actions.handleDeletionSuccess(buildJob());
    });

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(result.current.state.deletionStates["user-1"]).toEqual({
      jobId: "job-1",
      status: "running",
      error: undefined,
    });
    expect(result.current.state.isPolling).toBe(true);
  });

  it("start → cancel flow does not trigger polling", () => {
    const { result } = renderHook(() =>
      useUserDeletion({ onUserRemoved: mockOnUserRemoved }),
    );

    act(() => {
      result.current.actions.startDeletion({ id: "user-1", username: "alice" });
    });

    act(() => {
      result.current.actions.cancelDeletion();
    });

    expect(result.current.state.isPolling).toBe(false);
    expect(result.current.state.deletionStates).toEqual({});
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
