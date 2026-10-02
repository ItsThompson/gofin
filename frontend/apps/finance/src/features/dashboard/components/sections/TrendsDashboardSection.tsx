import type { TrendPoint } from "@gofin/core";
import { SectionErrorBoundary } from "@gofin/ui/components/SectionErrorBoundary";
import { dashboardApi } from "../../api";
import { useDashboardRequest } from "../../hooks/useDashboardRequest";
import { TrendsSection } from "../TrendsSection";
import type { DashboardDesktopSectionProps } from "./types";

interface TrendsDashboardSectionProps extends DashboardDesktopSectionProps {
  trendMonths: 6 | 12;
  onTrendMonthsChange: (months: 6 | 12) => void;
}

export function TrendsDashboardSection({ period, enabled, refreshVersion, currency, desktopVisible, trendMonths, onTrendMonthsChange }: TrendsDashboardSectionProps) {
  const trends = useDashboardRequest<readonly TrendPoint[]>(
    (signal, forceRefresh) => dashboardApi.getTrend(period.year, period.month, trendMonths, { signal, forceRefresh }).then((response) => response.trends),
    { enabled: enabled && desktopVisible, refreshVersion, requestKey: trendMonths, operation: "dashboard.trends", emptyWhen: (data) => data.length === 0, forceRefreshOnVersionChange: true },
  );
  const setTrendMonths = (months: 6 | 12) => {
    if (months === trendMonths) return;
    onTrendMonthsChange(months);
    if (!enabled || !desktopVisible) trends.retry();
  };
  if (!desktopVisible) return null;
  return (
    <section id="trends" data-outline-title="Trends">
      <SectionErrorBoundary sectionName="Monthly Trends">
        <TrendsSection trendData={trends.state.status === "success" ? [...trends.state.data] : []} trendState={trends.state} trendMonths={trendMonths} onToggle={setTrendMonths} onRetry={trends.retry} currency={currency} />
      </SectionErrorBoundary>
    </section>
  );
}
