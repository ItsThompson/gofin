import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { act } from "@testing-library/react";
import { usePolling } from "../src/hooks/usePolling";
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

    it("calls onError when fetcher throws", async () => {
      const onError = vi.fn();
      const error = new Error("fetch failed");
      const fetcher = vi.fn().mockRejectedValue(error);

      renderPolling({ fetcher, onError, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(onError).toHaveBeenCalledWith(error);
    });

    it("does not stop polling after an error", async () => {
      const fetcher = vi.fn()
        .mockRejectedValueOnce(new Error("err1"))
        .mockRejectedValueOnce(new Error("err2"))
        .mockResolvedValueOnce("ok");
      const onData = vi.fn();
      const onError = vi.fn();

      renderPolling({ fetcher, onData, onError, intervalMs: 1000 });

      await act(async () => {
        vi.advanceTimersByTime(3000);
      });
      expect(onError).toHaveBeenCalledTimes(2);
      expect(onData).toHaveBeenCalledWith("ok");
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
