import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { apiClient, ApiRequestError, consumeReturnToPath } from "@gofin/api";

const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock window.location for session expiry redirect tests
const originalLocation = window.location;

function mockWindowLocation() {
  Object.defineProperty(window, "location", {
    writable: true,
    value: { ...originalLocation, pathname: "/dashboard", href: "" },
  });
}

function restoreWindowLocation() {
  Object.defineProperty(window, "location", {
    writable: true,
    value: originalLocation,
  });
}

describe("apiClient", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    sessionStorage.clear();
  });

  describe("basic requests", () => {
    it("sends request with credentials and JSON headers", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: "test" }),
      });

      await apiClient("/api/test");

      expect(mockFetch).toHaveBeenCalledWith("/api/test", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      });
    });

    it("returns parsed JSON on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ user: { id: "123" } }),
      });

      const result = await apiClient<{ user: { id: string } }>("/api/test");
      expect(result.user.id).toBe("123");
    });

    it("returns undefined for 204 No Content", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      const result = await apiClient("/api/test");
      expect(result).toBeUndefined();
    });

    it("throws ApiRequestError on non-401 error", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: () =>
          Promise.resolve({
            code: "VALIDATION_ERROR",
            message: "Invalid input",
          }),
      });

      await expect(apiClient("/api/test")).rejects.toThrow(ApiRequestError);
      await expect(apiClient("/api/test")).rejects.toThrow();
    });
  });

  describe("silent token refresh on 401", () => {
    it("refreshes and retries on 401 from a regular API call", async () => {
      // First call: 401
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({ code: "UNAUTHORIZED", message: "Token expired" }),
      });

      // Refresh call: success
      mockFetch.mockResolvedValueOnce({ ok: true, status: 200 });

      // Retry of original call: success
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: "refreshed" }),
      });

      const result = await apiClient<{ data: string }>("/api/expenses");

      expect(result.data).toBe("refreshed");
      expect(mockFetch).toHaveBeenCalledTimes(3);

      // Verify the refresh call
      expect(mockFetch).toHaveBeenNthCalledWith(2, "/api/auth/refresh", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      });
    });

    it("does NOT attempt refresh for /api/auth/me (auth endpoint)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            code: "UNAUTHORIZED",
            message: "Not authenticated",
          }),
      });

      await expect(apiClient("/api/auth/me")).rejects.toThrow(ApiRequestError);
      // Only 1 call: no refresh attempt
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it("does NOT attempt refresh for /api/auth/login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            code: "INVALID_CREDENTIALS",
            message: "Invalid email or password",
          }),
      });

      await expect(apiClient("/api/auth/login", { method: "POST" })).rejects.toThrow();
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it("does NOT attempt refresh for /api/auth/register", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({ code: "UNAUTHORIZED", message: "Error" }),
      });

      await expect(apiClient("/api/auth/register", { method: "POST" })).rejects.toThrow();
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
  });

  describe("concurrent refresh prevention", () => {
    it("fires only one refresh request for concurrent 401s", async () => {
      // Both initial calls return 401
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        })
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        });

      // Single refresh call (shared by both)
      mockFetch.mockResolvedValueOnce({ ok: true, status: 200 });

      // Two retry calls
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: "a" }),
      });
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: "b" }),
      });

      const [resultA, resultB] = await Promise.all([
        apiClient<{ data: string }>("/api/expenses"),
        apiClient<{ data: string }>("/api/finance"),
      ]);

      expect(resultA.data).toBe("a");
      expect(resultB.data).toBe("b");

      // 2 initial calls + 1 refresh + 2 retries = 5 total
      expect(mockFetch).toHaveBeenCalledTimes(5);

      // Only one refresh call
      const refreshCalls = mockFetch.mock.calls.filter(
        (call) => call[0] === "/api/auth/refresh",
      );
      expect(refreshCalls).toHaveLength(1);
    });
  });

  describe("session expiry redirect", () => {
    beforeEach(() => {
      mockWindowLocation();
    });

    afterEach(() => {
      restoreWindowLocation();
    });

    it("saves path and redirects to /login?expired=true on refresh failure", async () => {
      // Initial call: 401
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
      });

      // Refresh fails
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

      await expect(apiClient("/api/expenses")).rejects.toThrow("session has expired");

      expect(sessionStorage.getItem("gofin_return_to")).toBe("/dashboard");
      expect(window.location.href).toBe("/login?expired=true");
    });

    it("throws SESSION_EXPIRED error after redirect", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
      });
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

      try {
        await apiClient("/api/expenses");
        expect.unreachable("should have thrown");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).code).toBe("SESSION_EXPIRED");
      }
    });
  });
});

describe("consumeReturnToPath", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("returns null when no path is stored", () => {
    expect(consumeReturnToPath()).toBeNull();
  });

  it("returns and clears the stored path", () => {
    sessionStorage.setItem("gofin_return_to", "/expenses");

    expect(consumeReturnToPath()).toBe("/expenses");
    expect(sessionStorage.getItem("gofin_return_to")).toBeNull();
  });

  it("returns null on second call (already consumed)", () => {
    sessionStorage.setItem("gofin_return_to", "/expenses");

    consumeReturnToPath();
    expect(consumeReturnToPath()).toBeNull();
  });
});
