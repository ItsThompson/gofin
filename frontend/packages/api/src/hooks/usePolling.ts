import { useEffect, useRef } from "react";

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
   * Called when the fetcher throws. Defaults to silently continuing
   * on the next tick (appropriate for transient network failures).
   */
  onError?: (error: unknown) => void;
}

/**
 * Generic polling hook that repeatedly calls a fetcher at a fixed interval.
 *
 * Handles:
 * - Start/stop via the `enabled` flag
 * - Auto-stop when `shouldStop` returns true
 * - Cleanup on unmount (no memory leaks)
 * - Stable callback references via ref (no stale closures)
 * - Silent error handling by default
 */
export function usePolling<T>({
  fetcher,
  enabled,
  intervalMs,
  onData,
  shouldStop,
  onError,
}: UsePollingOptions<T>): void {
  const callbacksRef = useRef({ fetcher, onData, shouldStop, onError });
  callbacksRef.current = { fetcher, onData, shouldStop, onError };

  useEffect(() => {
    if (!enabled) return;

    let intervalId: ReturnType<typeof setInterval> | null = null;

    const poll = async () => {
      try {
        const data = await callbacksRef.current.fetcher();

        callbacksRef.current.onData(data);

        if (callbacksRef.current.shouldStop?.(data)) {
          if (intervalId !== null) {
            clearInterval(intervalId);
            intervalId = null;
          }
        }
      } catch (error) {
        callbacksRef.current.onError?.(error);
      }
    };

    intervalId = setInterval(poll, intervalMs);

    return () => {
      if (intervalId !== null) {
        clearInterval(intervalId);
      }
    };
  }, [enabled, intervalMs]);
}
