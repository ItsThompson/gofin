import { useState, useCallback, useEffect, useRef } from "react";
import { ApiRequestError, usePolling } from "@gofin/api";
import { toast } from "sonner";
import { settingsApi } from "../api";
import type {
  ExportJob,
  ExportStatus,
  ExportDataState,
  ExportDataActions,
  ExportListResponse,
} from "../types";

const POLL_INTERVAL_MS = 5000;
const COOLDOWN_DAYS = 30;

function hasActiveJobs(jobs: ExportJob[]): boolean {
  return jobs.some(
    (job) => job.status === "pending" || job.status === "running",
  );
}

function computeCanExport(jobs: ExportJob[], nextExportDate: string | null): boolean {
  if (nextExportDate && new Date(nextExportDate) > new Date()) {
    return false;
  }

  if (hasActiveJobs(jobs)) {
    return false;
  }

  const latestNonFailed = jobs.find((job) => job.status !== "failed");
  if (!latestNonFailed) return true;

  const createdAt = new Date(latestNonFailed.createdAt);
  const cooldownEnd = new Date(createdAt);
  cooldownEnd.setDate(cooldownEnd.getDate() + COOLDOWN_DAYS);

  return new Date() >= cooldownEnd;
}

function computeNextExportDate(jobs: ExportJob[]): string | null {
  const latestNonFailed = jobs.find((job) => job.status !== "failed");
  if (!latestNonFailed) return null;

  const createdAt = new Date(latestNonFailed.createdAt);
  const cooldownEnd = new Date(createdAt);
  cooldownEnd.setDate(cooldownEnd.getDate() + COOLDOWN_DAYS);

  if (cooldownEnd > new Date()) {
    return cooldownEnd.toISOString();
  }
  return null;
}

export function useExportData(): { state: ExportDataState; actions: ExportDataActions } {
  const [status, setStatus] = useState<ExportStatus>("loading");
  const [jobs, setJobs] = useState<ExportJob[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [nextExportDate, setNextExportDate] = useState<string | null>(null);

  const mountedRef = useRef(true);

  const fetchJobs = useCallback(async () => {
    try {
      const response = await settingsApi.listExports(1, 50);
      if (!mountedRef.current) return;
      setJobs(response.data);
      setError(null);

      // Start polling immediately if active jobs are detected.
      if (hasActiveJobs(response.data)) {
        setStatus("polling");
      } else {
        setStatus("idle");
      }

      const computed = computeNextExportDate(response.data);
      setNextExportDate((prev) => prev ?? computed);
    } catch (err) {
      if (!mountedRef.current) return;
      if (err instanceof ApiRequestError) {
        setError(err.message);
      } else {
        setError("Failed to load export history.");
      }
      setStatus("error");
    }
  }, []);

  const handlePollData = useCallback((response: ExportListResponse) => {
    if (!mountedRef.current) return;
    setJobs(response.data);

    if (!hasActiveJobs(response.data)) {
      setStatus("idle");
    }
  }, []);

  usePolling<ExportListResponse>({
    fetcher: () => settingsApi.listExports(1, 50),
    enabled: status === "polling",
    intervalMs: POLL_INTERVAL_MS,
    onData: handlePollData,
    shouldStop: (response) => !hasActiveJobs(response.data),
  });

  const requestExport = useCallback(async () => {
    setStatus("creating");
    setError(null);

    try {
      const response = await settingsApi.createExport();
      if (!mountedRef.current) return;

      setJobs((prev) => {
        const exists = prev.some((job) => job.id === response.job.id);
        if (exists) return prev;
        return [response.job, ...prev];
      });

      toast.success(
        response.job.status === "pending"
          ? "Your data export is being prepared. You'll receive an email shortly."
          : "Your data export is already being prepared.",
      );

      setStatus("polling");
    } catch (err) {
      if (!mountedRef.current) return;

      if (err instanceof ApiRequestError && err.status === 429) {
        const isoMatch = err.message.match(/(\d{4}-\d{2}-\d{2})(T[\d:]+Z)?/);
        if (isoMatch) {
          const dateStr = isoMatch[2] ? isoMatch[0] : `${isoMatch[1]}T00:00:00Z`;
          setNextExportDate(dateStr);
        }
        setStatus("idle");
        return;
      }

      if (err instanceof ApiRequestError) {
        setError(err.message);
        toast.error(err.message);
      } else {
        setError("Failed to create export.");
        toast.error("Failed to create export. Please try again.");
      }
      setStatus("idle");
    }
  }, []);

  const refresh = useCallback(async () => {
    await fetchJobs();
  }, [fetchJobs]);

  // Initial fetch on mount
  useEffect(() => {
    mountedRef.current = true;
    fetchJobs();

    return () => {
      mountedRef.current = false;
    };
  }, [fetchJobs]);

  const canExport = computeCanExport(jobs, nextExportDate);

  return {
    state: { status, jobs, canExport, nextExportDate, error },
    actions: { requestExport, refresh },
  };
}
