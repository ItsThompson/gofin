import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { buildUser } from "@gofin/test-utils";
import { useLandingRedirect } from "../hooks/useLandingRedirect";
import {
  mockNavigate,
  checkAuth,
  setAuthStore,
  resetAuthMocks,
} from "./auth-mocks";

// The hook reads auth state from the store (which owns the /api/auth/me call)
// and navigates via react-router. Both are boundaries: mock them so the hook's
// own decision logic (the two effects) is exercised for real across the full
// branch table. getLandingPath (@gofin/core) is left real so the asserted path
// reflects the actual role -> path contract.
vi.mock("react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("react-router")>()),
  useNavigate: vi.fn(),
}));

vi.mock("@/stores/auth-store", () => ({
  useAuthStore: vi.fn(),
}));

beforeEach(resetAuthMocks);

describe("useLandingRedirect", () => {
  it("runs checkAuth once on mount", () => {
    setAuthStore({ isLoading: true, isAuthenticated: false, user: null });

    renderHook(() => useLandingRedirect());

    expect(checkAuth).toHaveBeenCalledTimes(1);
  });

  it("does not navigate while auth is still loading", () => {
    setAuthStore({ isLoading: true, isAuthenticated: false, user: null });

    renderHook(() => useLandingRedirect());

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("does not navigate for a resolved unauthenticated visitor", () => {
    setAuthStore({ isLoading: false, isAuthenticated: false, user: null });

    renderHook(() => useLandingRedirect());

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("redirects a resolved authenticated regular user to /dashboard with replace", () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "user" }),
    });

    renderHook(() => useLandingRedirect());

    expect(mockNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith("/dashboard", { replace: true });
  });

  it("redirects a resolved authenticated admin to /admin with replace", () => {
    setAuthStore({
      isLoading: false,
      isAuthenticated: true,
      user: buildUser({ role: "admin" }),
    });

    renderHook(() => useLandingRedirect());

    expect(mockNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith("/admin", { replace: true });
  });

  it("does not navigate when authenticated but the user is not yet resolved", () => {
    setAuthStore({ isLoading: false, isAuthenticated: true, user: null });

    renderHook(() => useLandingRedirect());

    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
