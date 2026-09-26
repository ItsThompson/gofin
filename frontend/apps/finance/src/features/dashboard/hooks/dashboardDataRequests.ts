import type {
  BudgetPeriod,
  CumulativeSpendPoint,
  Expense,
  HealthScore,
  HealthScoreConfigureBudget,
  HealthScoreTrendPoint,
  HistoricalComparison,
  PeriodSummary,
  ProRataSchedule,
  TagSpending,
  TrendPoint,
} from "@gofin/core";
import { dashboardApi } from "../api";
import type { DashboardSectionKey } from "../types";

export type DashboardSectionPayload =
  | { section: "summary"; data: PeriodSummary }
  | { section: "byTag"; data: readonly TagSpending[] }
  | { section: "cumulative"; data: readonly CumulativeSpendPoint[] }
  | { section: "recentExpenses"; data: readonly Expense[] }
  | { section: "comparison"; data: HistoricalComparison }
  | { section: "upcomingProRata"; data: readonly ProRataSchedule[] }
  | { section: "trends"; data: readonly TrendPoint[] }
  | { section: "healthScore"; data: HealthScore | HealthScoreConfigureBudget }
  | { section: "healthScoreTrend"; data: readonly HealthScoreTrendPoint[] };

export async function fetchDashboardSection(
  section: DashboardSectionKey,
  period: BudgetPeriod,
  trendMonths: 6 | 12,
  signal: AbortSignal,
): Promise<DashboardSectionPayload> {
  switch (section) {
    case "summary": {
      const response = await dashboardApi.getSummary(period.year, period.month, { signal });
      return { section, data: response.summary };
    }
    case "byTag": {
      const response = await dashboardApi.getTagSpending(period.year, period.month, { signal });
      return { section, data: response.tagSpending };
    }
    case "cumulative": {
      const response = await dashboardApi.getCumulative(period.year, period.month, { signal });
      return { section, data: response.points };
    }
    case "recentExpenses": {
      const response = await dashboardApi.getRecentExpenses(period.year, period.month, 5, { signal });
      return { section, data: response.data };
    }
    case "comparison": {
      const response = await dashboardApi.getComparison(period.year, period.month, { signal });
      return { section, data: response.comparison };
    }
    case "upcomingProRata": {
      const response = await dashboardApi.getUpcomingProRata({ signal });
      return { section, data: response.schedules };
    }
    case "trends": {
      const response = await dashboardApi.getTrend(period.year, period.month, trendMonths, { signal });
      return { section, data: response.trends };
    }
    case "healthScore": {
      const response = await dashboardApi.getHealthScore(period.year, period.month, { signal });
      return { section, data: response.healthScore };
    }
    case "healthScoreTrend": {
      const response = await dashboardApi.getHealthScoreTrend(period.year, period.month, 6, { signal });
      return { section, data: response.trends };
    }
  }
}
