import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useFormMutation } from "../src/hooks/useFormMutation";
import { ApiRequestError } from "../src/client";
import { NETWORK_ERROR_MESSAGE } from "../src/hooks/useApiToast";

describe("useFormMutation", () => {
  describe("success path", () => {
    it("sets submitting to true during operation", async () => {
      let resolveOp: (value: string) => void;
      const operation = () =>
        new Promise<string>((resolve) => {
          resolveOp = resolve;
        });

      const { result } = renderHook(() => useFormMutation<string>());

      expect(result.current.submitting).toBe(false);

      act(() => {
        result.current.submit(operation);
      });

      expect(result.current.submitting).toBe(true);

      await act(async () => {
        resolveOp!("done");
      });

      expect(result.current.submitting).toBe(false);
    });

    it("calls onSuccess with the result", async () => {
      const onSuccess = vi.fn();
      const { result } = renderHook(() =>
        useFormMutation<string>({ onSuccess }),
      );

      await act(async () => {
        result.current.submit(() => Promise.resolve("result-value"));
      });

      expect(onSuccess).toHaveBeenCalledWith("result-value");
    });

    it("does not set error on success", async () => {
      const { result } = renderHook(() => useFormMutation<string>());

      await act(async () => {
        result.current.submit(() => Promise.resolve("ok"));
      });

      expect(result.current.error).toBeNull();
    });

    it("clears previous error on new submit", async () => {
      const { result } = renderHook(() => useFormMutation<string>());

      // Trigger an error first
      await act(async () => {
        result.current.submit(() => Promise.reject(new Error("fail")));
      });

      expect(result.current.error).not.toBeNull();

      // Submit again successfully
      await act(async () => {
        result.current.submit(() => Promise.resolve("ok"));
      });

      expect(result.current.error).toBeNull();
    });
  });

  describe("ApiRequestError handling", () => {
    it("sets error to err.message for ApiRequestError", async () => {
      const apiError = new ApiRequestError(422, {
        code: "VALIDATION_ERROR",
        message: "Email is already taken",
      });

      const { result } = renderHook(() => useFormMutation<void>());

      await act(async () => {
        result.current.submit(() => Promise.reject(apiError));
      });

      expect(result.current.error).toBe("Email is already taken");
    });

    it("calls onError with the error message and the original error", async () => {
      const onError = vi.fn();
      const apiError = new ApiRequestError(400, {
        code: "BAD_REQUEST",
        message: "Invalid input",
      });

      const { result } = renderHook(() =>
        useFormMutation<void>({ onError }),
      );

      await act(async () => {
        result.current.submit(() => Promise.reject(apiError));
      });

      expect(onError).toHaveBeenCalledWith("Invalid input", apiError);
    });

    it("sets submitting to false after ApiRequestError", async () => {
      const apiError = new ApiRequestError(500, {
        code: "INTERNAL_ERROR",
        message: "Server error",
      });

      const { result } = renderHook(() => useFormMutation<void>());

      await act(async () => {
        result.current.submit(() => Promise.reject(apiError));
      });

      expect(result.current.submitting).toBe(false);
    });
  });

  describe("network error handling", () => {
    it("sets error to NETWORK_ERROR_MESSAGE for network errors", async () => {
      const networkError = new TypeError("Failed to fetch");

      const { result } = renderHook(() => useFormMutation<void>());

      await act(async () => {
        result.current.submit(() => Promise.reject(networkError));
      });

      expect(result.current.error).toBe(NETWORK_ERROR_MESSAGE);
    });

    it("calls onError with NETWORK_ERROR_MESSAGE", async () => {
      const onError = vi.fn();
      const networkError = new TypeError("Load failed");

      const { result } = renderHook(() =>
        useFormMutation<void>({ onError }),
      );

      await act(async () => {
        result.current.submit(() => Promise.reject(networkError));
      });

      expect(onError).toHaveBeenCalledWith(NETWORK_ERROR_MESSAGE, networkError);
    });
  });

  describe("unknown error handling", () => {
    it("sets generic message for unknown errors", async () => {
      const { result } = renderHook(() => useFormMutation<void>());

      await act(async () => {
        result.current.submit(() => Promise.reject(new Error("something")));
      });

      expect(result.current.error).toBe(
        "An unexpected error occurred. Please try again.",
      );
    });

    it("sets generic message for non-Error thrown values", async () => {
      const { result } = renderHook(() => useFormMutation<void>());

      await act(async () => {
        result.current.submit(() => Promise.reject("string error"));
      });

      expect(result.current.error).toBe(
        "An unexpected error occurred. Please try again.",
      );
    });

    it("calls onError with generic message for unknown errors", async () => {
      const onError = vi.fn();
      const { result } = renderHook(() =>
        useFormMutation<void>({ onError }),
      );

      await act(async () => {
        result.current.submit(() => Promise.reject(42));
      });

      expect(onError).toHaveBeenCalledWith(
        "An unexpected error occurred. Please try again.",
        42,
      );
    });
  });

  describe("clearError", () => {
    it("resets error to null", async () => {
      const { result } = renderHook(() => useFormMutation<void>());

      await act(async () => {
        result.current.submit(() => Promise.reject(new Error("fail")));
      });

      expect(result.current.error).not.toBeNull();

      act(() => {
        result.current.clearError();
      });

      expect(result.current.error).toBeNull();
    });

    it("is a no-op when error is already null", () => {
      const { result } = renderHook(() => useFormMutation<void>());

      expect(result.current.error).toBeNull();

      act(() => {
        result.current.clearError();
      });

      expect(result.current.error).toBeNull();
    });
  });

  describe("callbacks are optional", () => {
    it("works without any options", async () => {
      const { result } = renderHook(() => useFormMutation<string>());

      await act(async () => {
        result.current.submit(() => Promise.resolve("ok"));
      });

      expect(result.current.submitting).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it("works with empty options object", async () => {
      const { result } = renderHook(() => useFormMutation<string>({}));

      await act(async () => {
        result.current.submit(() => Promise.resolve("ok"));
      });

      expect(result.current.submitting).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it("does not throw when onSuccess is not provided and operation succeeds", async () => {
      const { result } = renderHook(() =>
        useFormMutation<string>({ onError: vi.fn() }),
      );

      await act(async () => {
        result.current.submit(() => Promise.resolve("ok"));
      });

      expect(result.current.error).toBeNull();
    });

    it("does not throw when onError is not provided and operation fails", async () => {
      const { result } = renderHook(() =>
        useFormMutation<void>({ onSuccess: vi.fn() }),
      );

      await act(async () => {
        result.current.submit(() => Promise.reject(new Error("oops")));
      });

      expect(result.current.error).toBe(
        "An unexpected error occurred. Please try again.",
      );
    });
  });

  describe("submit stability", () => {
    it("submit function reference is stable across re-renders", () => {
      const { result, rerender } = renderHook(() => useFormMutation<void>());

      const firstSubmit = result.current.submit;
      rerender();
      const secondSubmit = result.current.submit;

      expect(firstSubmit).toBe(secondSubmit);
    });

    it("clearError function reference is stable across re-renders", () => {
      const { result, rerender } = renderHook(() => useFormMutation<void>());

      const firstClear = result.current.clearError;
      rerender();
      const secondClear = result.current.clearError;

      expect(firstClear).toBe(secondClear);
    });
  });

  describe("double-submit guard", () => {
    it("ignores second submit while first is in-flight", async () => {
      let resolveFirst: () => void;
      const firstOp = vi.fn(
        () => new Promise<void>((resolve) => { resolveFirst = resolve; }),
      );
      const secondOp = vi.fn(() => Promise.resolve());

      const { result } = renderHook(() => useFormMutation<void>());

      act(() => {
        result.current.submit(firstOp);
      });

      expect(result.current.submitting).toBe(true);

      act(() => {
        result.current.submit(secondOp);
      });

      expect(secondOp).not.toHaveBeenCalled();

      await act(async () => {
        resolveFirst!();
      });

      expect(result.current.submitting).toBe(false);
      expect(firstOp).toHaveBeenCalledTimes(1);
      expect(secondOp).not.toHaveBeenCalled();
    });

    it("allows submit after previous completes", async () => {
      const firstOp = vi.fn(() => Promise.resolve());
      const secondOp = vi.fn(() => Promise.resolve());

      const { result } = renderHook(() => useFormMutation<void>());

      await act(async () => {
        result.current.submit(firstOp);
      });

      await act(async () => {
        result.current.submit(secondOp);
      });

      expect(firstOp).toHaveBeenCalledTimes(1);
      expect(secondOp).toHaveBeenCalledTimes(1);
    });

    it("allows submit after previous fails", async () => {
      const failOp = vi.fn(() => Promise.reject(new Error("fail")));
      const successOp = vi.fn(() => Promise.resolve());

      const { result } = renderHook(() => useFormMutation<void>());

      await act(async () => {
        result.current.submit(failOp);
      });

      expect(result.current.error).not.toBeNull();

      await act(async () => {
        result.current.submit(successOp);
      });

      expect(successOp).toHaveBeenCalledTimes(1);
      expect(result.current.error).toBeNull();
    });
  });
});
