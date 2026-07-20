import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useApiToast } from "../src/hooks/useApiToast";
import { ApiRequestError } from "../src/client";

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
  },
}));

import { toast } from "sonner";

describe("useApiToast", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("successful calls", () => {
    it("returns the result on success", async () => {
      const { result } = renderHook(() => useApiToast<string>());

      let value: string | undefined;
      await act(async () => {
        value = await result.current.call(() => Promise.resolve("data"));
      });

      expect(value).toBe("data");
    });

    it("does not show toast on success", async () => {
      const { result } = renderHook(() => useApiToast<string>());

      await act(async () => {
        await result.current.call(() => Promise.resolve("ok"));
      });

      expect(toast.error).not.toHaveBeenCalled();
    });
  });

  describe("error handling", () => {
    it("shows ApiRequestError message in toast", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(() =>
          Promise.reject(
            new ApiRequestError(400, {
              code: "BAD_REQUEST",
              message: "Invalid email format",
            }),
          ),
        );
      });

      expect(toast.error).toHaveBeenCalledWith(
        "Invalid email format",
        expect.any(Object),
      );
    });

    it("shows network error message for TypeError", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(() =>
          Promise.reject(new TypeError("Failed to fetch")),
        );
      });

      expect(toast.error).toHaveBeenCalledWith(
        "Connection lost. Check your internet and try again.",
        expect.any(Object),
      );
    });

    it("shows generic Error message in toast", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(() =>
          Promise.reject(new Error("Something broke")),
        );
      });

      expect(toast.error).toHaveBeenCalledWith(
        "Something broke",
        expect.any(Object),
      );
    });

    it("shows generic fallback for non-Error thrown values", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(() => Promise.reject(42));
      });

      expect(toast.error).toHaveBeenCalledWith(
        "An unexpected error occurred. Please try again.",
        expect.any(Object),
      );
    });

    it("returns undefined on error", async () => {
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

  describe("retry action", () => {
    it("includes retry action when retriable is true (default)", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(() => Promise.reject(new Error("fail")));
      });

      expect(toast.error).toHaveBeenCalledWith(
        "fail",
        expect.objectContaining({
          action: expect.objectContaining({ label: "Retry" }),
        }),
      );
    });

    it("does not include retry action when retriable is false", async () => {
      const { result } = renderHook(() => useApiToast({ retriable: false }));

      await act(async () => {
        await result.current.call(() => Promise.reject(new Error("fail")));
      });

      expect(toast.error).toHaveBeenCalledWith("fail", {});
    });

    it("retry action re-invokes the last operation", async () => {
      const operation = vi
        .fn()
        .mockRejectedValueOnce(new Error("first fail"))
        .mockResolvedValueOnce("success");

      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(operation);
      });

      // Get the retry onClick handler
      const toastCall = (toast.error as ReturnType<typeof vi.fn>).mock.calls[0];
      const retryAction = toastCall[1].action;

      await act(async () => {
        retryAction.onClick();
      });

      expect(operation).toHaveBeenCalledTimes(2);
    });
  });

  describe("silent mode", () => {
    it("does not show toast when silent option is true", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.call(
          () => Promise.reject(new Error("silent fail")),
          { silent: true },
        );
      });

      expect(toast.error).not.toHaveBeenCalled();
    });

    it("returns undefined in silent mode on error", async () => {
      const { result } = renderHook(() => useApiToast());

      let value: unknown;
      await act(async () => {
        value = await result.current.call(
          () => Promise.reject(new Error("fail")),
          { silent: true },
        );
      });

      expect(value).toBeUndefined();
    });
  });

  describe("callSilent", () => {
    it("does not show toast on error", async () => {
      const { result } = renderHook(() => useApiToast());

      await act(async () => {
        await result.current.callSilent(() =>
          Promise.reject(new Error("quiet")),
        );
      });

      expect(toast.error).not.toHaveBeenCalled();
    });

    it("returns the result on success", async () => {
      const { result } = renderHook(() => useApiToast<number>());

      let value: number | undefined;
      await act(async () => {
        value = await result.current.callSilent(() => Promise.resolve(42));
      });

      expect(value).toBe(42);
    });

    it("returns undefined on error", async () => {
      const { result } = renderHook(() => useApiToast());

      let value: unknown;
      await act(async () => {
        value = await result.current.callSilent(() =>
          Promise.reject(new Error("fail")),
        );
      });

      expect(value).toBeUndefined();
    });
  });
});
