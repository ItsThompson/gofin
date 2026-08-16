/** Status of an export job. */
export type ExportJobStatus = "pending" | "running" | "completed" | "failed";

/** A data export job resource returned by the datarights API. */
export interface ExportJob {
  id: string;
  userId: string;
  status: ExportJobStatus;
  createdAt: string;
  completedAt: string | null;
  fileSizeBytes: number | null;
  error: string | null;
}

/** Response from POST /api/datarights/exports (single job). */
export interface ExportJobResponse {
  job: ExportJob;
}

/** Response from GET /api/datarights/exports (paginated list). */
export interface ExportListResponse {
  data: ExportJob[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
}

/** Export lifecycle phases: exactly one is active at any time. */
export type ExportStatus = 'idle' | 'loading' | 'creating' | 'polling' | 'error';

/**
 * Save lifecycle for a settings form mutation. Exactly one kind is active;
 * the failure message lives on the `failed` kind, so `idle`/`saving`/`saved`
 * cannot carry a stale message.
 */
export type SaveStatus =
  | { kind: 'idle' }
  | { kind: 'saving' }
  | { kind: 'saved' }
  | { kind: 'failed'; message: string };

/** State returned by the useExportData hook. */
export interface ExportDataState {
  /** Current lifecycle phase. */
  status: ExportStatus;
  /** Export job history (populated after initial load). */
  jobs: ExportJob[];
  /** Whether the user can request a new export (cooldown + no active jobs). */
  canExport: boolean;
  /** ISO date when next export becomes available, or null if available now. */
  nextExportDate: string | null;
  /** Error message when status === 'error', or operational error in other states. */
  error: string | null;
}

/** Actions returned by the useExportData hook. */
export interface ExportDataActions {
  /** Request a new data export. Only callable when canExport === true. */
  requestExport: () => void;
  /** Re-fetch job list. */
  refresh: () => void;
}
