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

/** Error response from 429 rate-limited requests. */
export interface ExportRateLimitedResponse {
  code: string;
  message: string;
  retryAfter: string;
}

/** State returned by the useExportData hook. */
export interface ExportDataState {
  jobs: ExportJob[];
  loading: boolean;
  creating: boolean;
  error: string | null;
  canExport: boolean;
  nextExportDate: string | null;
}

/** Actions returned by the useExportData hook. */
export interface ExportDataActions {
  requestExport: () => Promise<void>;
  refresh: () => Promise<void>;
}
