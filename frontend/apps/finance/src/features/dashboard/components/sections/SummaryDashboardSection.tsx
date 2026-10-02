import { useEffect } from "react";
import type { PeriodSummary } from "@gofin/core";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { dashboardApi } from "../../api";
import { useDashboardRequest } from "../../hooks/useDashboardRequest";
import { SectionState } from "../SectionState";
import { SummaryBar } from "../widgets/SummaryBar";
import { CategoryGauges } from "../widgets/CategoryGauges";
import { PacingIndicator } from "../widgets/PacingIndicator";
import type { DashboardSummarySectionProps } from "./types";

export function SummaryDashboardSection({ period, enabled, refreshVersion, currency, summaryRetryVersion, onPeriodNotFound, onSuccess, onRetry }: DashboardSummarySectionProps) {
  const summary = useDashboardRequest<PeriodSummary>(
    (signal, forceRefresh) => dashboardApi.getSummary(period.year, period.month, { signal, forceRefresh }).then((response) => response.summary),
    {
      enabled,
      refreshVersion,
      retryVersion: summaryRetryVersion,
      operation: "dashboard.summary",
      onPeriodNotFound,
      loadingWhenDisabled: true,
      forceRefreshOnVersionChange: true,
    },
  );

  useEffect(() => {
    if (summary.state.status === "success") onSuccess();
  }, [onSuccess, summary.state.status]);

  return (
    <section id="summary" data-outline-title="Summary" className="space-y-6">
      <SectionState label="Summary" state={summary.state} onRetry={onRetry} emptyMessage="No summary data is available for this period.">
        {(data) => <>
          <SectionErrorBoundary sectionName="Summary"><SummaryBar budgetAmount={period.budgetAmount} totalSpent={data.totalSpent} remaining={data.remaining} daysLeft={data.daysInPeriod - data.daysElapsed} currency={currency} /></SectionErrorBoundary>
          <section id="budget-allocations" data-outline-title="Budget Allocations"><SectionErrorBoundary sectionName="Category Gauges"><CategoryGauges summary={data} currency={currency} /></SectionErrorBoundary></section>
          <div className="hidden md:grid md:grid-cols-2 md:gap-6">
            <section id="spending-pace" data-outline-title="Spending Pace"><SectionErrorBoundary sectionName="Spending Pace"><PacingIndicator summary={data} currency={currency} /></SectionErrorBoundary></section>
          </div>
        </>}
      </SectionState>
    </section>
  );
}
