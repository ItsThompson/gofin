import type { ExportJob, ExportJobStatus } from "../types";
import { formatFileSize } from "../utils/formatFileSize";

interface ExportHistoryTableProps {
  jobs: ExportJob[];
}

const STATUS_BADGE_STYLES: Record<ExportJobStatus, string> = {
  pending: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300",
  running: "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
  completed: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300",
  failed: "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300",
};

const STATUS_LABELS: Record<ExportJobStatus, string> = {
  pending: "Pending",
  running: "Running",
  completed: "Completed",
  failed: "Failed",
};

function formatDate(isoDate: string): string {
  return new Date(isoDate).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function StatusBadge({ status }: { status: ExportJobStatus }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_BADGE_STYLES[status]}`}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}

export function ExportHistoryTable({ jobs }: ExportHistoryTableProps) {
  if (jobs.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-muted-foreground">
        <p className="font-medium">No exports yet</p>
        <p className="mt-1 text-sm">
          Click &quot;Export My Data&quot; to download a copy of all your personal data.
        </p>
      </div>
    );
  }

  return (
    <>
      {/* Desktop table: visible at sm (640px) and above */}
      <div className="hidden sm:block">
        <table className="w-full text-sm" role="table">
          <thead>
            <tr className="border-b text-left text-muted-foreground">
              <th className="pb-2 font-medium">Date Requested</th>
              <th className="pb-2 font-medium">Status</th>
              <th className="pb-2 font-medium">Completed At</th>
              <th className="pb-2 font-medium">File Size</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {jobs.map((job) => (
              <tr key={job.id}>
                <td className="py-2.5">{formatDate(job.createdAt)}</td>
                <td className="py-2.5">
                  <StatusBadge status={job.status} />
                  {job.status === "failed" && job.error && (
                    <span className="ml-2 text-xs text-red-600 dark:text-red-400">
                      {job.error}
                    </span>
                  )}
                </td>
                <td className="py-2.5">
                  {job.completedAt ? formatDate(job.completedAt) : "—"}
                </td>
                <td className="py-2.5">
                  {job.fileSizeBytes != null ? formatFileSize(job.fileSizeBytes) : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Mobile card list: visible below sm (640px) */}
      <div className="flex flex-col gap-3 sm:hidden">
        {jobs.map((job) => (
          <div key={job.id} className="rounded-lg border p-3 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">
                {formatDate(job.createdAt)}
              </span>
              <StatusBadge status={job.status} />
            </div>
            {job.status === "failed" && job.error && (
              <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                {job.error}
              </p>
            )}
            <div className="mt-2 flex justify-between text-xs text-muted-foreground">
              <span>
                Completed: {job.completedAt ? formatDate(job.completedAt) : "—"}
              </span>
              <span>
                Size: {job.fileSizeBytes != null ? formatFileSize(job.fileSizeBytes) : "—"}
              </span>
            </div>
          </div>
        ))}
      </div>
    </>
  );
}
