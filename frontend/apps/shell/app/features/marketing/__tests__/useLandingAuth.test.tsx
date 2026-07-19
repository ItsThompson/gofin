import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { buildUser } from "@gofin/test-utils";
import { useLandingAuth } from "../hooks/useLandingAuth";
import { checkAuth, logout, setAuthStore, resetAuthMocks } from "./auth-mocks";

// The hook reads auth state from the store (which owns the /api/auth/me call).
// The store is the boundary: mock it so the hook's own behavior (run checkAuth
// on mount, expose the slice, never navigate) is exercised for real.
vi.mock("@/stores/auth-store", () => ({
  useAuthStore: vi.fn(),
}));

beforeEach(resetAuthMocks);

describe("useLandingAuth", () => {
  it("runs checkAuth once on mount", () => {
    setAuthStore({ isLoading: true, isAuthenticated: false, user: null });

    renderHook(() => useLandingAuth());

    expect(checkAuth).toHaveBeenCalledTimes(1);
  });

  it("returns the loading state before auth resolves", () => {
    setAuthStore({ isLoading: true, isAuthenticated: false, user: null });

    const { result } = renderHook(() => useLandingAuth());

    expect(result.current).toMatchObject({
      isLoading: true,
      isAuthenticated: false,
      user: null,
    });
  });

  it("returns the resolved unauthenticated state", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    const { result } = renderHook(() => useLandingAuth());

    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.user).toBeNull();
  });

  it("returns the authenticated user and the logout action", () => {
    const user = buildUser({ role: "user" });
    setAuthStore({ isLoading: false, isAuthenticated: true, user });

    const { result } = renderHook(() => useLandingAuth());

    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.user).toEqual(user);

    result.current.logout();
    expect(logout).toHaveBeenCalledTimes(1);
  });
});
