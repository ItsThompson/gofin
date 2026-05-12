import { Button } from "@gofin/ui/components/button";
import { Loader2, Download } from "lucide-react";
import { useExportData } from "../hooks/useExportData";
import { ExportHistoryTable } from "./ExportHistoryTable";
import type { ExportStatus } from "../types";

function formatCooldownDate(isoDate: string): string {
  return new Date(isoDate).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function getButtonContent(status: ExportStatus) {
  if (status === "creating") {
    return (
      <>
        <Loader2 className="size-4 animate-spin" />
        Exporting...
      </>
    );
  }
  if (status === "polling") {
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

export function ExportDataSection() {
  const { state, actions } = useExportData();

  if (state.status === "loading") {
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

  const isButtonDisabled =
    state.status === "creating" || state.status === "polling" || !state.canExport;

  const showCooldown =
    !state.canExport &&
    state.status !== "polling" &&
    state.status !== "creating" &&
    state.nextExportDate != null;

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
          {getButtonContent(state.status)}
        </Button>

        {showCooldown && state.nextExportDate && (
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
