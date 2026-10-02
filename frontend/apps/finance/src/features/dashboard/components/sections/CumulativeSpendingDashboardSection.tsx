import type { CumulativeSpendPoint } from "@gofin/core";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { dashboardApi } from "../../api";
import { useDashboardRequest } from "../../hooks/useDashboardRequest";
import { SectionState } from "../SectionState";
import { CumulativeSpendChart } from "../widgets/CumulativeSpendChart";
import type { DashboardDesktopSectionProps } from "./types";

export function CumulativeSpendingDashboardSection({ period, enabled, refreshVersion, currency, desktopVisible }: DashboardDesktopSectionProps) {
  const cumulative = useDashboardRequest<readonly CumulativeSpendPoint[]>(
    (signal, forceRefresh) => dashboardApi.getCumulative(period.year, period.month, { signal, forceRefresh }).then((response) => response.points),
    { enabled: enabled && desktopVisible, refreshVersion, operation: "dashboard.cumulative", emptyWhen: (data) => data.length === 0, forceRefreshOnVersionChange: true },
  );
  if (!desktopVisible) return null;
  if (cumulative.state.status !== "success") {
    return <SectionState label="Cumulative spending" state={cumulative.state} onRetry={cumulative.retry} emptyMessage="No cumulative spending data is available.">{() => null}</SectionState>;
  }
  return (
    <section id="cumulative-spending" data-outline-title="Cumulative Spending">
      <SectionState label="Cumulative spending" state={cumulative.state} onRetry={cumulative.retry} emptyMessage="No cumulative spending data is available.">
        {(data) => <SectionErrorBoundary sectionName="Cumulative Spending"><CumulativeSpendChart data={[...data]} currency={currency} /></SectionErrorBoundary>}
      </SectionState>
    </section>
  );
}
