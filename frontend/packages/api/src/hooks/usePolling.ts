import { useEffect, useRef } from "react";

/** Consecutive transport failures tolerated before polling gives up. */
export const DEFAULT_MAX_CONSECUTIVE_FAILURES = 3;

export interface UsePollingOptions<T> {
  /** Async function that fetches the data on each tick. */
  fetcher: () => Promise<T>;
  /** Whether polling is active. Setting to false stops the interval. */
  enabled: boolean;
  /** Interval between polls in milliseconds. */
  intervalMs: number;
  /**
   * Called with the fetched data on each successful tick.
   * Use this to update external state or trigger side effects.
   */
  onData: (data: T) => void;
  /**
   * When provided, polling auto-stops if this returns true for fetched data.
   * Useful for terminal state detection (e.g., job completed/failed).
   */
  shouldStop?: (data: T) => boolean;
  /**
   * Consecutive fetcher failures tolerated before polling stops.
   * Defaults to DEFAULT_MAX_CONSECUTIVE_FAILURES. A single success resets the
   * count, so transient failures do not consume the budget.
   */
  maxConsecutiveFailures?: number;
  /**
   * Called once, with the last error, when polling stops because the failure
   * budget ran out. Callers own telling the user, because a poll that has given
   * up leaves whatever it was tracking in an unknown state.
   */
  onFailureLimitReached?: (error: unknown) => void;
}

/**
 * Generic polling hook that repeatedly calls a fetcher at a fixed interval.
 *
 * Handles:
 * - Start/stop via the `enabled` flag
 * - Auto-stop when `shouldStop` returns true
 * - Auto-stop after `maxConsecutiveFailures` consecutive fetcher failures
 * - Cleanup on unmount (no memory leaks)
 * - Stable callback references via ref (no stale closures)
 *
 * Individual failures are deliberately not surfaced: a 2.5-second interval
 * against a failing endpoint would emit a signal per tick for as long as the tab
 * stays open. Only the terminal failure reaches the caller.
 */
export function usePolling<T>({
  fetcher,
  enabled,
  intervalMs,
  onData,
  shouldStop,
  maxConsecutiveFailures = DEFAULT_MAX_CONSECUTIVE_FAILURES,
  onFailureLimitReached,
}: UsePollingOptions<T>): void {
  const callbacksRef = useRef({
    fetcher,
    onData,
    shouldStop,
    onFailureLimitReached,
  });
  callbacksRef.current = { fetcher, onData, shouldStop, onFailureLimitReached };

  useEffect(() => {
    if (!enabled) return;

    let intervalId: ReturnType<typeof setInterval> | null = null;
    let consecutiveFailures = 0;

    const stop = () => {
      if (intervalId !== null) {
        clearInterval(intervalId);
        intervalId = null;
      }
    };

    const poll = async () => {
      try {
        const data = await callbacksRef.current.fetcher();
        consecutiveFailures = 0;

        callbacksRef.current.onData(data);

        if (callbacksRef.current.shouldStop?.(data)) {
          stop();
        }
      } catch (error) {
        consecutiveFailures += 1;
        if (consecutiveFailures < maxConsecutiveFailures) return;

        stop();
        callbacksRef.current.onFailureLimitReached?.(error);
      }
    };

    intervalId = setInterval(poll, intervalMs);

    return stop;
  }, [enabled, intervalMs, maxConsecutiveFailures]);
}
