import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { act } from "@testing-library/react";
import {
  usePolling,
  DEFAULT_MAX_CONSECUTIVE_FAILURES,
} from "../src/hooks/usePolling";
import type { UsePollingOptions } from "../src/hooks/usePolling";

describe("usePolling", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function renderPolling<T>(overrides: Partial<UsePollingOptions<T>> = {}) {
    const defaults: UsePollingOptions<T> = {
      fetcher: vi.fn().mockResolvedValue(undefined) as () => Promise<T>,
      enabled: true,
      intervalMs: 1000,
      onData: vi.fn(),
      ...overrides,
    };
    return { ...renderHook((props) => usePolling(props), { initialProps: defaults }), options: defaults };
  }

  describe("start/stop lifecycle", () => {
    it("does not poll when enabled is false", async () => {
      const fetcher = vi.fn().mockResolvedValue("data");
      renderPolling({ fetcher, enabled: false });

      await act(async () => {
        vi.advanceTimersByTime(5000);
      });

      expect(fetcher).not.toHaveBeenCalled();
    });

    it("polls at the specified interval when enabled", async () => {
      const fetcher = vi.fn().mockResolvedValue("data");
      renderPolling({ fetcher, enabled: true, intervalMs: 2000 });

      expect(fetcher).not.toHaveBeenCalled();

      await act(async () => {
        vi.advanceTimersByTime(2000);
      });
      expect(fetcher).toHaveBeenCalledTimes(1);

      await act(async () => {
        vi.advanceTimersByTime(2000);
      });
      expect(fetcher).toHaveBeenCalledTimes(2);
    });

    it("stops polling when enabled changes from true to false", async () => {
      const fetcher = vi.fn().mockResolvedValue("data");
      const { rerender, options } = renderPolling({ fetcher, enabled: true, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(fetcher).toHaveBeenCalledTimes(1);

      rerender({ ...options, enabled: false });
      fetcher.mockClear();

      await act(async () => {
        vi.advanceTimersByTime(3000);
      });
      expect(fetcher).not.toHaveBeenCalled();
    });

    it("cleans up interval on unmount", async () => {
      const fetcher = vi.fn().mockResolvedValue("data");
      const { unmount } = renderPolling({ fetcher, enabled: true, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(fetcher).toHaveBeenCalledTimes(1);

      unmount();
      fetcher.mockClear();

      await act(async () => {
        vi.advanceTimersByTime(3000);
      });
      expect(fetcher).not.toHaveBeenCalled();
    });

    it("restarts polling when intervalMs changes", async () => {
      const fetcher = vi.fn().mockResolvedValue("data");
      const { rerender, options } = renderPolling({ fetcher, enabled: true, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(fetcher).toHaveBeenCalledTimes(1);

      rerender({ ...options, intervalMs: 3000 });
      fetcher.mockClear();

      // Old interval (1s) should NOT fire
      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(fetcher).not.toHaveBeenCalled();

      // New interval (3s) should fire
      await act(async () => {
        vi.advanceTimersByTime(2000);
      });
      expect(fetcher).toHaveBeenCalledTimes(1);
    });
  });

  describe("data handling", () => {
    it("calls onData with fetched data on each tick", async () => {
      const onData = vi.fn();
      const fetcher = vi.fn()
        .mockResolvedValueOnce("first")
        .mockResolvedValueOnce("second");

      renderPolling({ fetcher, onData, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onData).toHaveBeenCalledWith("first");

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onData).toHaveBeenCalledWith("second");
    });

    it("does not call onData when fetcher throws", async () => {
      const onData = vi.fn();
      const fetcher = vi.fn().mockRejectedValue(new Error("network"));

      renderPolling({ fetcher, onData, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onData).not.toHaveBeenCalled();
    });
  });

  describe("shouldStop", () => {
    it("stops polling when shouldStop returns true", async () => {
      const fetcher = vi.fn()
        .mockResolvedValueOnce({ done: false })
        .mockResolvedValueOnce({ done: true });
      const onData = vi.fn();
      const shouldStop = (data: { done: boolean }) => data.done;

      renderPolling({ fetcher, onData, shouldStop, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onData).toHaveBeenCalledWith({ done: false });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onData).toHaveBeenCalledWith({ done: true });

      // No further polling
      fetcher.mockClear();
      onData.mockClear();
      await act(async () => {
        vi.advanceTimersByTime(3000);
      });
      expect(fetcher).not.toHaveBeenCalled();
      expect(onData).not.toHaveBeenCalled();
    });

    it("continues polling when shouldStop returns false", async () => {
      const fetcher = vi.fn().mockResolvedValue({ done: false });
      const shouldStop = (data: { done: boolean }) => data.done;

      renderPolling({ fetcher, shouldStop, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(3000);
      });
      expect(fetcher).toHaveBeenCalledTimes(3);
    });

    it("calls onData before stopping (data is delivered even on terminal tick)", async () => {
      const onData = vi.fn();
      const fetcher = vi.fn().mockResolvedValue("terminal");
      const shouldStop = () => true;

      renderPolling({ fetcher, onData, shouldStop, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onData).toHaveBeenCalledWith("terminal");
    });
  });

  describe("error handling", () => {
    it("continues polling on error by default (silent)", async () => {
      const fetcher = vi.fn()
        .mockRejectedValueOnce(new Error("transient"))
        .mockResolvedValueOnce("recovered");
      const onData = vi.fn();

      renderPolling({ fetcher, onData, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onData).not.toHaveBeenCalled();

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onData).toHaveBeenCalledWith("recovered");
    });

    it("keeps polling while failures stay below the limit", async () => {
      const fetcher = vi.fn()
        .mockRejectedValueOnce(new Error("err1"))
        .mockRejectedValueOnce(new Error("err2"))
        .mockResolvedValueOnce("ok");
      const onData = vi.fn();
      const onFailureLimitReached = vi.fn();

      renderPolling({
        fetcher,
        onData,
        onFailureLimitReached,
        intervalMs: 1000,
      });

      await act(async () => {
        vi.advanceTimersByTime(3000);
      });
      expect(onFailureLimitReached).not.toHaveBeenCalled();
      expect(onData).toHaveBeenCalledWith("ok");
    });
  });

  describe("consecutive failure limit", () => {
    it("stops after the default number of consecutive failures", async () => {
      const lastError = new Error("still down");
      const fetcher = vi.fn()
        .mockRejectedValueOnce(new Error("down 1"))
        .mockRejectedValueOnce(new Error("down 2"))
        .mockRejectedValue(lastError);
      const onFailureLimitReached = vi.fn();

      renderPolling({ fetcher, onFailureLimitReached, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(3000);
      });
      expect(fetcher).toHaveBeenCalledTimes(
        DEFAULT_MAX_CONSECUTIVE_FAILURES,
      );
      expect(onFailureLimitReached).toHaveBeenCalledTimes(1);
      expect(onFailureLimitReached).toHaveBeenCalledWith(lastError);

      // The interval is gone: no further ticks, and no further callbacks.
      fetcher.mockClear();
      await act(async () => {
        vi.advanceTimersByTime(10000);
      });
      expect(fetcher).not.toHaveBeenCalled();
      expect(onFailureLimitReached).toHaveBeenCalledTimes(1);
    });

    it("honors a caller-supplied limit", async () => {
      const fetcher = vi.fn().mockRejectedValue(new Error("down"));
      const onFailureLimitReached = vi.fn();

      renderPolling({
        fetcher,
        onFailureLimitReached,
        maxConsecutiveFailures: 2,
        intervalMs: 1000,
      });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onFailureLimitReached).not.toHaveBeenCalled();

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onFailureLimitReached).toHaveBeenCalledTimes(1);
      expect(fetcher).toHaveBeenCalledTimes(2);
    });

    it("resets the failure count on any success", async () => {
      const fetcher = vi.fn()
        .mockRejectedValueOnce(new Error("err1"))
        .mockRejectedValueOnce(new Error("err2"))
        .mockResolvedValueOnce("recovered")
        .mockRejectedValueOnce(new Error("err3"))
        .mockRejectedValueOnce(new Error("err4"))
        .mockResolvedValueOnce("recovered again");
      const onData = vi.fn();
      const onFailureLimitReached = vi.fn();

      renderPolling({
        fetcher,
        onData,
        onFailureLimitReached,
        intervalMs: 1000,
      });

      await act(async () => {
        vi.advanceTimersByTime(6000);
      });

      // Six ticks with two runs of two failures: the budget never ran out.
      expect(fetcher).toHaveBeenCalledTimes(6);
      expect(onFailureLimitReached).not.toHaveBeenCalled();
      expect(onData).toHaveBeenNthCalledWith(1, "recovered");
      expect(onData).toHaveBeenNthCalledWith(2, "recovered again");
    });

    it("starts a fresh failure budget when polling restarts", async () => {
      const fetcher = vi.fn().mockRejectedValue(new Error("down"));
      const onFailureLimitReached = vi.fn();
      const { rerender, options } = renderPolling({
        fetcher,
        onFailureLimitReached,
        maxConsecutiveFailures: 2,
        intervalMs: 1000,
      });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(fetcher).toHaveBeenCalledTimes(1);

      rerender({ ...options, enabled: false });
      rerender({ ...options, enabled: true });
      fetcher.mockClear();

      // One failure carried over would trip the limit on the first new tick.
      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onFailureLimitReached).not.toHaveBeenCalled();

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onFailureLimitReached).toHaveBeenCalledTimes(1);
    });

    it("reports the terminal failure once when requests outlive the interval", async () => {
      // The dangerous outage shape: a hanging endpoint behind a gateway timeout,
      // so several requests are in flight when the budget runs out.
      const fetcher = vi.fn(
        (): Promise<string> =>
          new Promise((_resolve, reject) => {
            setTimeout(() => reject(new Error("slow failure")), 3200);
          }),
      );
      const onFailureLimitReached = vi.fn();

      renderPolling<string>({
        fetcher,
        onFailureLimitReached,
        intervalMs: 1000,
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(10000);
      });

      // More ticks fired than the budget allows, so rejections land after
      // polling stopped. Each one used to re-fire the callback.
      expect(fetcher.mock.calls.length).toBeGreaterThan(
        DEFAULT_MAX_CONSECUTIVE_FAILURES,
      );
      expect(onFailureLimitReached).toHaveBeenCalledTimes(1);
    });

    it("delivers nothing from a tick that outlived the stop", async () => {
      const onData = vi.fn();
      const fetcher = vi.fn(
        (): Promise<string> =>
          new Promise((resolve) => {
            setTimeout(() => resolve("late"), 2500);
          }),
      );
      const { unmount } = renderPolling<string>({
        fetcher,
        onData,
        intervalMs: 1000,
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000);
      });
      expect(fetcher).toHaveBeenCalledTimes(1);

      unmount();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });
      expect(onData).not.toHaveBeenCalled();
    });
  });

  describe("callback stability", () => {
    it("uses latest callbacks without restarting the interval", async () => {
      const firstOnData = vi.fn();
      const secondOnData = vi.fn();
      const fetcher = vi.fn().mockResolvedValue("data");

      const { rerender, options } = renderPolling({
        fetcher,
        onData: firstOnData,
        intervalMs: 1000,
      });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(firstOnData).toHaveBeenCalledWith("data");

      // Update onData callback (should NOT restart the interval)
      rerender({ ...options, onData: secondOnData });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(secondOnData).toHaveBeenCalledWith("data");
      expect(firstOnData).toHaveBeenCalledTimes(1);
    });
  });
});
