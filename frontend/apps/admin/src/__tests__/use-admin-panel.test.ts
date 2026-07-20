import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { useAdminPanel } from "@/hooks/useAdminPanel";
import type { User } from "@gofin/core";

const mockFetch = vi.fn();
global.fetch = mockFetch;

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const mockAdmin: User = {
  id: "admin-1",
  username: "admin",
  email: "admin@gofin.local",
  role: "admin",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockUsers = [
  { id: "user-1", username: "alice", email: "alice@example.com", role: "user", createdAt: "2026-01-15T00:00:00Z" },
  { id: "admin-1", username: "admin", email: "admin@gofin.local", role: "admin", createdAt: "2026-01-01T00:00:00Z" },
  { id: "user-2", username: "bob", email: "bob@example.com", role: "user", createdAt: "2026-02-01T00:00:00Z" },
];

function mockFetchSuccess() {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: () => Promise.resolve({ users: mockUsers }),
  });
}

function mockFetchError() {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status: 500,
    json: () => Promise.resolve({ code: "INTERNAL_SERVER_ERROR", message: "Server error" }),
  });
}

describe("useAdminPanel", () => {
  const mockOnAssume = vi.fn();

  beforeEach(() => {
    mockFetch.mockReset();
    mockOnAssume.mockReset();
  });

  it("starts in loading state with empty users", () => {
    mockFetch.mockReturnValueOnce(new Promise(() => {}));

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    expect(result.current.state.loadState).toBe("loading");
    expect(result.current.state.users).toEqual([]);
    expect(result.current.state.assumingUserId).toBeNull();
  });

  it("transitions to success state after fetching users", async () => {
    mockFetchSuccess();

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("success");
    });

    expect(result.current.state.users).toEqual(mockUsers);
  });

  it("transitions to error state on fetch failure", async () => {
    mockFetchError();

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("error");
    });

    expect(result.current.state.users).toEqual([]);
  });

  it("retry re-fetches users and resets loadState to loading", async () => {
    mockFetchError();

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("error");
    });

    mockFetchSuccess();

    act(() => {
      result.current.actions.retry();
    });

    expect(result.current.state.loadState).toBe("loading");

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("success");
    });

    expect(result.current.state.users).toEqual(mockUsers);
  });

  it("handleAssume sets assumingUserId and calls onAssumeIdentity", async () => {
    mockFetchSuccess();
    mockOnAssume.mockResolvedValueOnce(undefined);

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("success");
    });

    act(() => {
      result.current.actions.handleAssume("user-1");
    });

    expect(result.current.state.assumingUserId).toBe("user-1");
    expect(mockOnAssume).toHaveBeenCalledWith("user-1");
  });

  it("handleAssume clears assumingUserId on failure", async () => {
    mockFetchSuccess();
    mockOnAssume.mockRejectedValueOnce(new Error("Network error"));

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("success");
    });

    await act(async () => {
      result.current.actions.handleAssume("user-1");
    });

    expect(result.current.state.assumingUserId).toBeNull();
  });

  it("exposes deletion state from useUserDeletion", async () => {
    mockFetchSuccess();

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("success");
    });

    expect(result.current.state.deletion.deletingUser).toBeNull();
    expect(result.current.state.deletion.deletionStates).toEqual({});
    expect(result.current.state.deletion.isPolling).toBe(false);
  });

  it("exposes deletion actions from useUserDeletion", async () => {
    mockFetchSuccess();

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("success");
    });

    act(() => {
      result.current.actions.deletion.startDeletion({ id: "user-1", username: "alice" });
    });

    expect(result.current.state.deletion.deletingUser).toEqual({ id: "user-1", username: "alice" });

    act(() => {
      result.current.actions.deletion.cancelDeletion();
    });

    expect(result.current.state.deletion.deletingUser).toBeNull();
  });

  it("onUserRemoved filters user from the list when deletion completes", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockFetchSuccess();

    const { result } = renderHook(() =>
      useAdminPanel({ currentUser: mockAdmin, onAssumeIdentity: mockOnAssume }),
    );

    await waitFor(() => {
      expect(result.current.state.loadState).toBe("success");
    });

    expect(result.current.state.users).toHaveLength(3);

    // Start deletion and trigger success (starts polling)
    act(() => {
      result.current.actions.deletion.startDeletion({ id: "user-1", username: "alice" });
    });

    act(() => {
      result.current.actions.deletion.handleDeletionSuccess({
        id: "job-1",
        userId: "user-1",
        status: "pending",
        error: null,
        createdAt: "2026-05-10T00:00:00Z",
        completedAt: null,
      });
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({
        id: "job-1",
        userId: "user-1",
        status: "completed",
        error: null,
        createdAt: "2026-05-10T00:00:00Z",
        completedAt: "2026-05-10T00:01:00Z",
      }),
    });

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });

    expect(result.current.state.users).toHaveLength(2);
    expect(result.current.state.users.find((user) => user.id === "user-1")).toBeUndefined();

    vi.useRealTimers();
  });
});
