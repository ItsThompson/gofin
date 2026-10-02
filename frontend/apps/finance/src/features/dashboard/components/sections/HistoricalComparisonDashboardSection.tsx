import type { HistoricalComparison } from "@gofin/core";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { dashboardApi } from "../../api";
import { useDashboardRequest } from "../../hooks/useDashboardRequest";
import { SectionState } from "../SectionState";
import { HistoricalComparisonWidget } from "../widgets/HistoricalComparisonWidget";
import type { DashboardDesktopSectionProps } from "./types";

export function HistoricalComparisonDashboardSection({
  period,
  enabled,
  refreshVersion,
  currency,
  desktopVisible,
}: DashboardDesktopSectionProps) {
  const comparison = useDashboardRequest<HistoricalComparison>(
    (signal, forceRefresh) =>
      dashboardApi
        .getComparison(period.year, period.month, { signal, forceRefresh })
        .then((response) => response.comparison),
    {
      enabled: enabled && desktopVisible,
      refreshVersion,
      operation: "dashboard.comparison",
      periodNotFoundAsEmpty: true,
      forceRefreshOnVersionChange: true,
    },
  );
  if (!desktopVisible) return null;
  if (comparison.state.status !== "success") {
    return (
      <SectionState
        label="Historical comparison"
        state={comparison.state}
        onRetry={comparison.retry}
        emptyMessage="Not enough data for comparison."
      >
        {() => null}
      </SectionState>
    );
  }
  return (
    <section
      id="historical-comparison"
      data-outline-title="Historical Comparison"
    >
      <SectionState
        label="Historical comparison"
        state={comparison.state}
        onRetry={comparison.retry}
        emptyMessage="Not enough data for comparison."
      >
        {(data) => (
          <SectionErrorBoundary sectionName="Historical Comparison">
            <HistoricalComparisonWidget comparison={data} currency={currency} />
          </SectionErrorBoundary>
        )}
      </SectionState>
    </section>
  );
}
