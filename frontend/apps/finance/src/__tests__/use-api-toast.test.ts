import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useApiToast, isNetworkError, NETWORK_ERROR_MESSAGE } from "@gofin/types";
import { ApiRequestError } from "@gofin/types";
import { toast } from "sonner";

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
  },
}));

const mockToastError = vi.mocked(toast.error);

describe("useApiToast", () => {
  beforeEach(() => {
    mockToastError.mockReset();
  });

  describe("call", () => {
    it("returns result on success", async () => {
      const { result } = renderHook(() => useApiToast<string>());

      let value: string | undefined;
      await act(async () => {
        value = await result.current.call(() => Promise.resolve("data"));
      });

      expect(value).toBe("data");
      expect(mockToastError).not.toHaveBeenCalled();
    });

    it("shows toast with ApiRequestError message on failure", async () => {
      const { result } = renderHook(() => useApiToast());
      const error = new ApiRequestError(400, {
        code: "VALIDATION_ERROR",
        message: "Amount must be greater than 0",
      });

      let value: unknown;
      await act(async () => {
        value = await result.current.call(() => Promise.reject(error));
      });

      expect(value).toBeUndefined();
      expect(mockToastError).toHaveBeenCalledWith(
        "Amount must be greater than 0",
        expect.objectContaining({
          action: expect.objectContaining({ label: "Retry" }),
        }),
      );
    });

    it("shows network error message for TypeError fetch failures", async () => {
      const { result } = renderHook(() => useApiToast());
      const error = new TypeError("Failed to fetch");

      await act(async () => {
        await result.current.call(() => Promise.reject(error));
      });

      expect(mockToastError).toHaveBeenCalledWith(
        NETWORK_ERROR_MESSAGE,
        expect.any(Object),
      );
    });

    it("shows generic message for unknown errors", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(() => Promise.reject("string error"));
      });

      expect(mockToastError).toHaveBeenCalledWith(
        "An unexpected error occurred. Please try again.",
        expect.any(Object),
      );
    });

    it("includes Retry action when retriable is true (default)", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(() =>
          Promise.reject(new Error("fail")),
        );
      });

      const callArgs = mockToastError.mock.calls[0];
      expect(callArgs[1]).toHaveProperty("action");
      expect((callArgs[1] as { action: { label: string } }).action.label).toBe(
        "Retry",
      );
    });

    it("omits Retry action when retriable is false", async () => {
      const { result } = renderHook(() =>
        useApiToast({ retriable: false }),
      );

      await act(async () => {
        await result.current.call(() =>
          Promise.reject(new Error("fail")),
        );
      });

      const callArgs = mockToastError.mock.calls[0];
      expect(callArgs[1]).not.toHaveProperty("action");
    });

    it("returns undefined on failure", async () => {
      const { result } = renderHook(() => useApiToast());

      let value: unknown;
      await act(async () => {
        value = await result.current.call(() =>
          Promise.reject(new Error("fail")),
        );
      });

      expect(value).toBeUndefined();
    });
  });

  describe("callSilent", () => {
    it("returns result on success without toast", async () => {
      const { result } = renderHook(() => useApiToast<string>());

      let value: string | undefined;
      await act(async () => {
        value = await result.current.callSilent(() =>
          Promise.resolve("data"),
        );
      });

      expect(value).toBe("data");
      expect(mockToastError).not.toHaveBeenCalled();
    });

    it("suppresses toast on failure", async () => {
      const { result } = renderHook(() => useApiToast());

      let value: unknown;
      await act(async () => {
        value = await result.current.callSilent(() =>
          Promise.reject(new Error("fail")),
        );
      });

      expect(value).toBeUndefined();
      expect(mockToastError).not.toHaveBeenCalled();
    });
  });
});

describe("isNetworkError", () => {
  it("returns true for TypeError with fetch message", () => {
    expect(isNetworkError(new TypeError("Failed to fetch"))).toBe(true);
  });

  it("returns true for TypeError with network message", () => {
    expect(isNetworkError(new TypeError("NetworkError when attempting to fetch resource"))).toBe(true);
  });

  it("returns true for TypeError with load failed message", () => {
    expect(isNetworkError(new TypeError("Load failed"))).toBe(true);
  });

  it("returns false for non-TypeError", () => {
    expect(isNetworkError(new Error("Failed to fetch"))).toBe(false);
  });

  it("returns false for unrelated TypeError", () => {
    expect(isNetworkError(new TypeError("Cannot read property"))).toBe(false);
  });

  it("returns false for non-error values", () => {
    expect(isNetworkError("string")).toBe(false);
    expect(isNetworkError(null)).toBe(false);
    expect(isNetworkError(undefined)).toBe(false);
  });
});
