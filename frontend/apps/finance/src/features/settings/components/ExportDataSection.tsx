import { Button } from "@gofin/ui/components/button";
import { Loader2, Download } from "lucide-react";
import { useExportData } from "../hooks/useExportData";
import { ExportHistoryTable } from "./ExportHistoryTable";

function formatCooldownDate(isoDate: string): string {
  return new Date(isoDate).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export function ExportDataSection() {
  const { state, actions } = useExportData();

  const hasActiveJob = state.jobs.some(
    (job) => job.status === "pending" || job.status === "running",
  );

  const isInCooldown = !state.canExport && !hasActiveJob && state.nextExportDate != null;
  const isButtonDisabled = state.creating || !state.canExport || hasActiveJob;

  function getButtonContent() {
    if (state.creating) {
      return (
        <>
          <Loader2 className="size-4 animate-spin" />
          Exporting...
        </>
      );
    }
    if (hasActiveJob) {
      return (
        <>
          <Loader2 className="size-4 animate-spin" />
          Export in progress...
        </>
      );
    }
    return (
      <>
        <Download className="size-4" />
        Export My Data
      </>
    );
  }

  if (state.loading) {
    return (
      <div className="space-y-4">
        <h3 className="text-base font-medium">Data Export</h3>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" />
          Loading export history...
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-base font-medium">Data Export</h3>
        <p className="mt-1 text-sm text-muted-foreground">
          Export all your personal data as a ZIP of CSV files. The export will be
          emailed to your registered address.
        </p>
      </div>

      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <Button
          type="button"
          onClick={actions.requestExport}
          disabled={isButtonDisabled}
          className="w-full sm:w-auto"
        >
          {getButtonContent()}
        </Button>

        {isInCooldown && state.nextExportDate && (
          <span className="text-sm text-muted-foreground">
            Next export available: {formatCooldownDate(state.nextExportDate)}
          </span>
        )}
      </div>

      <div>
        <h4 className="mb-3 text-sm font-medium">Export History</h4>
        <ExportHistoryTable jobs={state.jobs} />
      </div>
    </div>
  );
}
