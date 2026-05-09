import { useState, useCallback, useEffect, useRef } from "react";
import { ApiRequestError } from "@gofin/api";
import { toast } from "sonner";
import { settingsApi } from "../api";
import type {
  ExportJob,
  ExportDataState,
  ExportDataActions,
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
  const [jobs, setJobs] = useState<ExportJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nextExportDate, setNextExportDate] = useState<string | null>(null);

  const pollIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const mountedRef = useRef(true);

  const fetchJobs = useCallback(async () => {
    try {
      const response = await settingsApi.listExports(1, 50);
      if (!mountedRef.current) return;
      setJobs(response.data);
      setError(null);

      const computed = computeNextExportDate(response.data);
      setNextExportDate((prev) => prev ?? computed);
    } catch (err) {
      if (!mountedRef.current) return;
      if (err instanceof ApiRequestError) {
        setError(err.message);
      } else {
        setError("Failed to load export history.");
      }
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, []);

  const startPolling = useCallback(() => {
    if (pollIntervalRef.current) return;

    pollIntervalRef.current = setInterval(async () => {
      try {
        const response = await settingsApi.listExports(1, 50);
        if (!mountedRef.current) return;
        setJobs(response.data);

        if (!hasActiveJobs(response.data) && pollIntervalRef.current) {
          clearInterval(pollIntervalRef.current);
          pollIntervalRef.current = null;
        }
      } catch {
        // Silently retry on next poll interval
      }
    }, POLL_INTERVAL_MS);
  }, []);

  const stopPolling = useCallback(() => {
    if (pollIntervalRef.current) {
      clearInterval(pollIntervalRef.current);
      pollIntervalRef.current = null;
    }
  }, []);

  const requestExport = useCallback(async () => {
    setCreating(true);
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

      startPolling();
    } catch (err) {
      if (!mountedRef.current) return;

      if (err instanceof ApiRequestError && err.status === 429) {
        // Extract date from error message (format: "...after 2026-06-08." or ISO)
        const isoMatch = err.message.match(/(\d{4}-\d{2}-\d{2})(T[\d:]+Z)?/);
        if (isoMatch) {
          const dateStr = isoMatch[2] ? isoMatch[0] : `${isoMatch[1]}T00:00:00Z`;
          setNextExportDate(dateStr);
        }

        // No error toast for 429: just update state
        return;
      }

      if (err instanceof ApiRequestError) {
        setError(err.message);
        toast.error(err.message);
      } else {
        setError("Failed to create export.");
        toast.error("Failed to create export. Please try again.");
      }
    } finally {
      if (mountedRef.current) {
        setCreating(false);
      }
    }
  }, [startPolling]);

  const refresh = useCallback(async () => {
    await fetchJobs();
  }, [fetchJobs]);

  // Initial fetch on mount
  useEffect(() => {
    mountedRef.current = true;
    fetchJobs();

    return () => {
      mountedRef.current = false;
      stopPolling();
    };
  }, [fetchJobs, stopPolling]);

  // Start/stop polling based on job states
  useEffect(() => {
    if (hasActiveJobs(jobs)) {
      startPolling();
    } else {
      stopPolling();
    }
  }, [jobs, startPolling, stopPolling]);

  const canExport = computeCanExport(jobs, nextExportDate);

  return {
    state: { jobs, loading, creating, error, canExport, nextExportDate },
    actions: { requestExport, refresh },
  };
}
