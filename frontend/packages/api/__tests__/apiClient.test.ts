import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { apiClient, ApiRequestError } from "../src/client";

describe("apiClient", () => {
  const originalLocation = window.location;
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    sessionStorage.clear();
    Object.defineProperty(window, "location", {
      writable: true,
      value: { pathname: "/dashboard", href: "" },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    Object.defineProperty(window, "location", {
      writable: true,
      value: originalLocation,
    });
  });

  describe("successful requests", () => {
    it("returns parsed JSON on successful response", async () => {
      fetchMock.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 1, name: "Test" }),
      });

      const result = await apiClient<{ id: number; name: string }>("/api/data");

      expect(result).toEqual({ id: 1, name: "Test" });
    });

    it("includes credentials and content-type headers", async () => {
      fetchMock.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({}),
      });

      await apiClient("/api/data");

      expect(fetchMock).toHaveBeenCalledWith("/api/data", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      });
    });

    it("passes through custom options", async () => {
      fetchMock.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({}),
      });

      await apiClient("/api/data", {
        method: "POST",
        body: JSON.stringify({ value: "test" }),
      });

      expect(fetchMock).toHaveBeenCalledWith("/api/data", {
        method: "POST",
        body: JSON.stringify({ value: "test" }),
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      });
    });

    it("returns undefined for 204 No Content responses", async () => {
      fetchMock.mockResolvedValue({
        ok: true,
        status: 204,
      });

      const result = await apiClient<void>("/api/resource", {
        method: "DELETE",
      });

      expect(result).toBeUndefined();
    });
  });

  describe("error responses", () => {
    it("throws ApiRequestError with parsed error body", async () => {
      fetchMock.mockResolvedValue({
        ok: false,
        status: 422,
        json: () =>
          Promise.resolve({
            code: "VALIDATION_ERROR",
            message: "Invalid input",
            fields: { email: "Required" },
          }),
      });

      await expect(apiClient("/api/data")).rejects.toThrow(ApiRequestError);

      try {
        await apiClient("/api/data");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).status).toBe(422);
        expect((error as ApiRequestError).code).toBe("VALIDATION_ERROR");
        expect((error as ApiRequestError).message).toBe("Invalid input");
        expect((error as ApiRequestError).fields).toEqual({ email: "Required" });
      }
    });

    it("throws with fallback error when response body is not JSON", async () => {
      fetchMock.mockResolvedValue({
        ok: false,
        status: 500,
        statusText: "Internal Server Error",
        json: () => Promise.reject(new Error("not json")),
      });

      try {
        await apiClient("/api/data");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).code).toBe("UNKNOWN_ERROR");
        expect((error as ApiRequestError).message).toBe("Internal Server Error");
      }
    });

    it("uses generic message when statusText is empty", async () => {
      fetchMock.mockResolvedValue({
        ok: false,
        status: 500,
        statusText: "",
        json: () => Promise.reject(new Error("not json")),
      });

      try {
        await apiClient("/api/data");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).message).toBe(
          "An unexpected error occurred",
        );
      }
    });
  });

  describe("401 handling on auth endpoints", () => {
    it("throws immediately for /api/auth/login 401 (no refresh)", async () => {
      fetchMock.mockResolvedValue({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            code: "INVALID_CREDENTIALS",
            message: "Wrong password",
          }),
      });

      try {
        await apiClient("/api/auth/login");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).code).toBe("INVALID_CREDENTIALS");
      }

      // Should not attempt refresh
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it("throws immediately for /api/auth/register 401", async () => {
      fetchMock.mockResolvedValue({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            code: "AUTH_ERROR",
            message: "Registration failed",
          }),
      });

      try {
        await apiClient("/api/auth/register");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).code).toBe("AUTH_ERROR");
      }

      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it("throws immediately for /api/auth/me sub-paths", async () => {
      fetchMock.mockResolvedValue({
        ok: false,
        status: 401,
        json: () =>
          Promise.resolve({
            code: "AUTH_ERROR",
            message: "Not authorized",
          }),
      });

      try {
        await apiClient("/api/auth/me/password");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).code).toBe("AUTH_ERROR");
      }

      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
  });

  describe("401 with token refresh", () => {
    it("attempts refresh and retries on 401 for non-auth endpoints", async () => {
      fetchMock
        // First call: 401
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        })
        // Refresh call: success
        .mockResolvedValueOnce({ ok: true })
        // Retry: success
        .mockResolvedValueOnce({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: "refreshed" }),
        });

      const result = await apiClient<{ data: string }>("/api/dashboard");

      expect(result).toEqual({ data: "refreshed" });
      expect(fetchMock).toHaveBeenCalledTimes(3);
      expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/auth/refresh", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      });
    });

    it("redirects to login when refresh fails", async () => {
      fetchMock
        // First call: 401
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        })
        // Refresh call: failure
        .mockResolvedValueOnce({ ok: false, status: 401 });

      await expect(apiClient("/api/dashboard")).rejects.toThrow(
        ApiRequestError,
      );

      expect(window.location.href).toBe("/login?expired=true");
      expect(sessionStorage.getItem("gofin_return_to")).toBe("/dashboard");
    });

    it("throws SESSION_EXPIRED error when refresh fails", async () => {
      fetchMock
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        })
        .mockResolvedValueOnce({ ok: false, status: 401 });

      try {
        await apiClient("/api/dashboard");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).code).toBe("SESSION_EXPIRED");
        expect((error as ApiRequestError).status).toBe(401);
      }
    });

    it("throws if retry after refresh also fails", async () => {
      fetchMock
        // First call: 401
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        })
        // Refresh: success
        .mockResolvedValueOnce({ ok: true })
        // Retry: still fails
        .mockResolvedValueOnce({
          ok: false,
          status: 403,
          json: () =>
            Promise.resolve({ code: "FORBIDDEN", message: "No access" }),
        });

      try {
        await apiClient("/api/restricted");
      } catch (error) {
        expect(error).toBeInstanceOf(ApiRequestError);
        expect((error as ApiRequestError).status).toBe(403);
        expect((error as ApiRequestError).code).toBe("FORBIDDEN");
      }
    });

    it("handles network error during refresh", async () => {
      fetchMock
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        })
        // Refresh call throws network error
        .mockRejectedValueOnce(new TypeError("Failed to fetch"));

      await expect(apiClient("/api/dashboard")).rejects.toThrow(
        ApiRequestError,
      );

      expect(window.location.href).toBe("/login?expired=true");
    });

    it("deduplicates concurrent refresh requests", async () => {
      fetchMock
        // First call: 401
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        })
        // Second call: 401
        .mockResolvedValueOnce({
          ok: false,
          status: 401,
          json: () =>
            Promise.resolve({ code: "UNAUTHORIZED", message: "Expired" }),
        })
        // Single refresh call
        .mockResolvedValueOnce({ ok: true })
        // Retry first
        .mockResolvedValueOnce({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: "a" }),
        })
        // Retry second
        .mockResolvedValueOnce({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: "b" }),
        });

      const [resultA, resultB] = await Promise.all([
        apiClient<{ data: string }>("/api/a"),
        apiClient<{ data: string }>("/api/b"),
      ]);

      expect(resultA).toEqual({ data: "a" });
      expect(resultB).toEqual({ data: "b" });

      // Only one refresh call should have been made
      const refreshCalls = fetchMock.mock.calls.filter(
        (call: string[]) => call[0] === "/api/auth/refresh",
      );
      expect(refreshCalls).toHaveLength(1);
    });
  });
});
