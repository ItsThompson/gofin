import type { HealthScore, HealthScoreConfigureBudget, HealthScoreTrendPoint } from "@gofin/core";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { dashboardApi } from "../../api";
import { useDashboardRequest } from "../../hooks/useDashboardRequest";
import { SectionState } from "../SectionState";
import { HealthScoreCard } from "../widgets/HealthScoreCard";
import type { DashboardSectionProps } from "./types";

export function HealthScoreDashboardSection({ period, enabled, refreshVersion }: DashboardSectionProps) {
  const score = useDashboardRequest<HealthScore | HealthScoreConfigureBudget>(
    (signal, forceRefresh) => dashboardApi.getHealthScore(period.year, period.month, { signal, forceRefresh }).then((response) => response.healthScore),
    { enabled, refreshVersion, operation: "dashboard.healthScore", forceRefreshOnVersionChange: true },
  );
  const trend = useDashboardRequest<readonly HealthScoreTrendPoint[]>(
    (signal, forceRefresh) => dashboardApi.getHealthScoreTrend(period.year, period.month, 6, { signal, forceRefresh }).then((response) => response.trends),
    { enabled, refreshVersion, operation: "dashboard.healthScoreTrend", emptyWhen: (data) => data.length === 0, forceRefreshOnVersionChange: true },
  );

  return (
    <section id="health-score" data-outline-title="Health Score">
      <SectionState label="Health Score" state={score.state} onRetry={score.retry} emptyMessage="No health score is available for this period.">
        {(data) => <SectionErrorBoundary sectionName="Health Score"><HealthScoreCard score={data} trend={trend.state.status === "success" ? [...trend.state.data] : null} /></SectionErrorBoundary>}
      </SectionState>
      <SectionState label="Health score trend" state={trend.state} onRetry={trend.retry} emptyMessage="No health score trend is available.">
        {() => null}
      </SectionState>
    </section>
  );
}
