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
import { expenseSuggestionsApi } from "../../expense-autocomplete/api";
import type { DashboardSectionKey } from "../types";
import {
  isActiveExpenseSuggestion,
  type ActiveExpenseSuggestion,
} from "../components/widgets/expenseFrecencyChartData";

export type DashboardSectionPayload =
  | { section: "summary"; data: PeriodSummary }
  | { section: "byTag"; data: readonly TagSpending[] }
  | { section: "cumulative"; data: readonly CumulativeSpendPoint[] }
  | { section: "recentExpenses"; data: readonly Expense[] }
  | { section: "comparison"; data: HistoricalComparison }
  | { section: "upcomingProRata"; data: readonly ProRataSchedule[] }
  | { section: "trends"; data: readonly TrendPoint[] }
  | { section: "healthScore"; data: HealthScore | HealthScoreConfigureBudget }
  | { section: "healthScoreTrend"; data: readonly HealthScoreTrendPoint[] }
  | { section: "suggestions"; data: readonly ActiveExpenseSuggestion[] };

async function fetchSuggestions(
  pageSize: number,
  signal: AbortSignal,
  forceRefresh: boolean,
): Promise<readonly ActiveExpenseSuggestion[]> {
  const suggestions: ActiveExpenseSuggestion[] = [];
  let page = 1;
  let hasMore = true;
  while (suggestions.length < pageSize && hasMore && !signal.aborted) {
    const response = await expenseSuggestionsApi.getSuggestions(page, pageSize, {
      signal,
      forceRefresh,
    });
    suggestions.push(...response.data.filter(isActiveExpenseSuggestion));
    hasMore = response.hasMore;
    page += 1;
  }
  return suggestions.slice(0, pageSize);
}

export async function fetchDashboardSection(
  section: DashboardSectionKey,
  period: BudgetPeriod,
  trendMonths: 6 | 12,
  signal: AbortSignal,
  forceRefresh = false,
): Promise<DashboardSectionPayload> {
  switch (section) {
    case "summary": {
      const response = await dashboardApi.getSummary(period.year, period.month, { signal, forceRefresh });
      return { section, data: response.summary };
    }
    case "byTag": {
      const response = await dashboardApi.getTagSpending(period.year, period.month, { signal, forceRefresh });
      return { section, data: response.tagSpending };
    }
    case "cumulative": {
      const response = await dashboardApi.getCumulative(period.year, period.month, { signal, forceRefresh });
      return { section, data: response.points };
    }
    case "recentExpenses": {
      const response = await dashboardApi.getRecentExpenses(period.year, period.month, 5, { signal, forceRefresh });
      return { section, data: response.data };
    }
    case "comparison": {
      const response = await dashboardApi.getComparison(period.year, period.month, { signal, forceRefresh });
      return { section, data: response.comparison };
    }
    case "upcomingProRata": {
      const response = await dashboardApi.getUpcomingProRata({ signal, forceRefresh });
      return { section, data: response.schedules };
    }
    case "trends": {
      const response = await dashboardApi.getTrend(period.year, period.month, trendMonths, { signal, forceRefresh });
      return { section, data: response.trends };
    }
    case "healthScore": {
      const response = await dashboardApi.getHealthScore(period.year, period.month, { signal, forceRefresh });
      return { section, data: response.healthScore };
    }
    case "healthScoreTrend": {
      const response = await dashboardApi.getHealthScoreTrend(period.year, period.month, 6, { signal, forceRefresh });
      return { section, data: response.trends };
    }
    case "suggestions": {
      const suggestions = await fetchSuggestions(10, signal, forceRefresh);
      return { section, data: suggestions };
    }
  }
}
