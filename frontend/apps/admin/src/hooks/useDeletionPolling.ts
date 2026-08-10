import { useCallback } from "react";
import { apiClient, usePolling } from "@gofin/api";
import type { DeletionJobResponse, DeletionStatus } from "../components/DeleteUserDialog/types";

export interface UseDeletionPollingOptions {
  jobId: string;
  enabled: boolean;
  intervalMs?: number;
  onStatusChange: (status: DeletionStatus, error?: string) => void;
  onCompleted: () => void;
  onFailed: (error: string) => void;
  /**
   * Called once, with the last transport error, when polling gives up because
   * the status endpoint kept failing. The deletion's real outcome is unknown at
   * that point: it is not onFailed.
   */
  onStatusUnavailable: (error: unknown) => void;
}

const DEFAULT_INTERVAL_MS = 2500;

function isTerminalStatus(job: DeletionJobResponse): boolean {
  return job.status === "completed" || job.status === "failed";
}

export function useDeletionPolling({
  jobId,
  enabled,
  intervalMs = DEFAULT_INTERVAL_MS,
  onStatusChange,
  onCompleted,
  onFailed,
  onStatusUnavailable,
}: UseDeletionPollingOptions): void {
  const handleData = useCallback(
    (job: DeletionJobResponse) => {
      onStatusChange(job.status, job.error ?? undefined);

      if (job.status === "completed") {
        onCompleted();
      } else if (job.status === "failed") {
        onFailed(job.error ?? "Unknown error");
      }
    },
    [onStatusChange, onCompleted, onFailed],
  );

  usePolling<DeletionJobResponse>({
    fetcher: () => apiClient<DeletionJobResponse>(`/api/datarights/deletions/${jobId}`),
    enabled: enabled && !!jobId,
    intervalMs,
    onData: handleData,
    shouldStop: isTerminalStatus,
    onFailureLimitReached: onStatusUnavailable,
  });
}
