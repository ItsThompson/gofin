import { useEffect, useRef } from "react";
import { apiClient } from "@gofin/api";
import type { DeletionJobResponse } from "../components/DeleteUserDialog/types";

export interface UseDeletionPollingOptions {
  jobId: string;
  enabled: boolean;
  intervalMs?: number;
  onStatusChange: (status: DeletionStatus, error?: string) => void;
  onCompleted: () => void;
  onFailed: (error: string) => void;
}

const DEFAULT_INTERVAL_MS = 2500;

export function useDeletionPolling({
  jobId,
  enabled,
  intervalMs = DEFAULT_INTERVAL_MS,
  onStatusChange,
  onCompleted,
  onFailed,
}: UseDeletionPollingOptions): void {
  const callbacksRef = useRef({ onStatusChange, onCompleted, onFailed });
  callbacksRef.current = { onStatusChange, onCompleted, onFailed };

  useEffect(() => {
    if (!enabled || !jobId) return;

    let intervalId: ReturnType<typeof setInterval> | null = null;

    const poll = async () => {
      try {
        const job = await apiClient<DeletionJobResponse>(
          `/api/datarights/deletions/${jobId}`,
        );
        const status = job.status;

        callbacksRef.current.onStatusChange(status, job.error ?? undefined);

        if (status === "completed") {
          if (intervalId !== null) {
            clearInterval(intervalId);
            intervalId = null;
          }
          callbacksRef.current.onCompleted();
        } else if (status === "failed") {
          if (intervalId !== null) {
            clearInterval(intervalId);
            intervalId = null;
          }
          callbacksRef.current.onFailed(job.error ?? "Unknown error");
        }
      } catch {
        // Network error during polling: silently continue on next tick
      }
    };

    intervalId = setInterval(poll, intervalMs);

    return () => {
      if (intervalId !== null) {
        clearInterval(intervalId);
      }
    };
  }, [jobId, enabled, intervalMs]);
}
