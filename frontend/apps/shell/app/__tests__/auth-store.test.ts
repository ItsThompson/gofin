import { describe, it, expect, vi, beforeEach } from "vitest";
import { useAuthStore } from "@/stores/auth-store";
import type { User } from "@gofin/types";

const mockUser: User = {
  id: "user-123",
  username: "testuser",
  email: "test@example.com",
  role: "user",
  currency: "USD",
  hasCompletedOnboarding: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const mockAdminUser: User = {
  ...mockUser,
  id: "admin-456",
  username: "admin",
  email: "admin@example.com",
  role: "admin",
};

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

function resetStore() {
  useAuthStore.setState({
    user: null,
    isAuthenticated: false,
    isAdmin: false,
    isAssuming: false,
    originalAdminUser: null,
    isLoading: true,
  });
}

describe("auth store", () => {
  beforeEach(() => {
    resetStore();
    mockFetch.mockReset();
  });

  describe("checkAuth", () => {
    it("sets authenticated state on successful /api/auth/me", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: mockUser }),
      });

      await useAuthStore.getState().checkAuth();

      const state = useAuthStore.getState();
      expect(state.user).toEqual(mockUser);
      expect(state.isAuthenticated).toBe(true);
      expect(state.isAdmin).toBe(false);
      expect(state.isLoading).toBe(false);
    });

    it("sets isAdmin when user has admin role", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: mockAdminUser }),
      });

      await useAuthStore.getState().checkAuth();

      const state = useAuthStore.getState();
      expect(state.isAdmin).toBe(true);
      expect(state.user?.role).toBe("admin");
    });

    it("clears state on 401 (not authenticated)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            code: "UNAUTHORIZED",
            message: "Authentication required",
          }),
      });

      await useAuthStore.getState().checkAuth();

      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.isAuthenticated).toBe(false);
      expect(state.isLoading).toBe(false);
    });

    it("clears state on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      await useAuthStore.getState().checkAuth();

      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.isAuthenticated).toBe(false);
      expect(state.isLoading).toBe(false);
    });
  });

  describe("login", () => {
    it("sets authenticated state on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: mockUser }),
      });

      const user = await useAuthStore.getState().login("test@example.com", "Password1");

      expect(user).toEqual(mockUser);
      const state = useAuthStore.getState();
      expect(state.user).toEqual(mockUser);
      expect(state.isAuthenticated).toBe(true);
      expect(state.isLoading).toBe(false);
    });

    it("sends correct request body", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: mockUser }),
      });

      await useAuthStore.getState().login("test@example.com", "Password1");

      expect(mockFetch).toHaveBeenCalledWith("/api/auth/login", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: "test@example.com", password: "Password1" }),
      });
    });

    it("throws ApiRequestError on invalid credentials", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            code: "INVALID_CREDENTIALS",
            message: "Invalid email or password",
          }),
      });

      await expect(
        useAuthStore.getState().login("test@example.com", "wrong"),
      ).rejects.toThrow("Invalid email or password");

      const state = useAuthStore.getState();
      expect(state.isAuthenticated).toBe(false);
    });
  });

  describe("logout", () => {
    it("clears authenticated state", async () => {
      // First set up authenticated state
      useAuthStore.setState({
        user: mockUser,
        isAuthenticated: true,
        isAdmin: false,
        isLoading: false,
      });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      await useAuthStore.getState().logout();

      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.isAuthenticated).toBe(false);
      expect(state.isLoading).toBe(false);
    });

    it("clears state even when server call fails", async () => {
      useAuthStore.setState({
        user: mockUser,
        isAuthenticated: true,
        isAdmin: false,
        isLoading: false,
      });

      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      await useAuthStore.getState().logout();

      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.isAuthenticated).toBe(false);
    });
  });

  describe("state transitions", () => {
    it("starts in loading state", () => {
      resetStore();
      expect(useAuthStore.getState().isLoading).toBe(true);
    });

    it("transitions from unauthenticated → authenticated → logged out", async () => {
      // Start: unauthenticated
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ code: "UNAUTHORIZED", message: "No session" }),
      });
      await useAuthStore.getState().checkAuth();
      expect(useAuthStore.getState().isAuthenticated).toBe(false);

      // Login
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: mockUser }),
      });
      await useAuthStore.getState().login("test@example.com", "Password1");
      expect(useAuthStore.getState().isAuthenticated).toBe(true);
      expect(useAuthStore.getState().user).toEqual(mockUser);

      // Logout
      mockFetch.mockResolvedValueOnce({ ok: true, status: 204 });
      await useAuthStore.getState().logout();
      expect(useAuthStore.getState().isAuthenticated).toBe(false);
      expect(useAuthStore.getState().user).toBeNull();
    });
  });
});
