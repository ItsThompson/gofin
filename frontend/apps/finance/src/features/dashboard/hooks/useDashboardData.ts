import { useState, useEffect, useCallback } from "react";
import { useApiToast } from "@gofin/api";
import type {
  PeriodSummary,
  SummaryResponse,
  TagSpending,
  TagSpendingResponse,
  CumulativeSpendPoint,
  CumulativeSpendResponse,
  Expense,
  HistoricalComparison,
  ProRataSchedule,
  TrendPoint,
  HealthScore,
  HealthScoreConfigureBudget,
  HealthScoreTrendPoint,
} from "@gofin/core";
import type { PaginatedResponse } from "@gofin/core";
import { dashboardApi } from "../api";

// The health-score sparkline always shows the last 6 monthly scores.
const HEALTH_SCORE_TREND_MONTHS = 6;

export interface DashboardData {
  summary: PeriodSummary | null;
  tagSpending: TagSpending[];
  cumulativeData: CumulativeSpendPoint[];
  recentExpenses: Expense[];
  comparison: HistoricalComparison | null;
  upcomingProRata: ProRataSchedule[];
  trendData: TrendPoint[] | null;
  healthScore: HealthScore | HealthScoreConfigureBudget | null;
  healthScoreTrend: HealthScoreTrendPoint[] | null;
}

export const EMPTY_DASHBOARD_DATA: DashboardData = {
  summary: null,
  tagSpending: [],
  cumulativeData: [],
  recentExpenses: [],
  comparison: null,
  upcomingProRata: [],
  trendData: null,
  healthScore: null,
  healthScoreTrend: null,
};

export interface DashboardDataResult {
  data: DashboardData;
  loading: boolean;
  refresh: () => void;
  trendMonths: 6 | 12;
  setTrendMonths: (months: 6 | 12) => void;
}

export function useDashboardData(
  year: number,
  month: number,
): DashboardDataResult {
  const [data, setData] = useState<DashboardData>(EMPTY_DASHBOARD_DATA);
  const [loading, setLoading] = useState(true);
  const [trendMonths, setTrendMonths] = useState<6 | 12>(6);
  const { call: toastCall } = useApiToast();

  const fetchDashboardData = useCallback(async () => {
    setLoading(true);
    setData(EMPTY_DASHBOARD_DATA);

    // Fetch each section independently so a single endpoint failure
    // doesn't prevent the rest of the dashboard from rendering.
    // Critical sections use toastCall (shows error toast to user).
    // Non-critical sections (comparison, proRata) fail silently with fallbacks.
    const [summaryRes, tagRes, cumulativeRes, expensesRes, comparisonRes, upcomingRes, healthRes] =
      await Promise.all([
        toastCall(() => dashboardApi.getSummary(year, month)) as Promise<SummaryResponse | undefined>,
        toastCall(() => dashboardApi.getTagSpending(year, month)) as Promise<TagSpendingResponse | undefined>,
        toastCall(() => dashboardApi.getCumulative(year, month)) as Promise<CumulativeSpendResponse | undefined>,
        toastCall(() => dashboardApi.getRecentExpenses(year, month, 5)) as Promise<PaginatedResponse<Expense> | undefined>,
        dashboardApi.getComparison(year, month).catch(() => null),
        dashboardApi.getUpcomingProRata().catch(() => ({ schedules: [] as ProRataSchedule[] })),
        dashboardApi.getHealthScore(year, month).catch(() => null),
      ]);

    setData((prev) => ({
      ...prev,
      summary: summaryRes?.summary ?? null,
      tagSpending: tagRes?.tagSpending ?? [],
      cumulativeData: cumulativeRes?.points ?? [],
      recentExpenses: expensesRes?.data ?? [],
      comparison: comparisonRes?.comparison ?? null,
      upcomingProRata: upcomingRes?.schedules ?? [],
      healthScore: healthRes?.healthScore ?? null,
    }));

    setLoading(false);
  }, [year, month, toastCall]);

  const fetchTrendData = useCallback(async () => {
    const trendRes = await dashboardApi
      .getTrend(year, month, trendMonths)
      .catch(() => null);
    setData(prev => ({ ...prev, trendData: trendRes?.trends ?? null }));
  }, [year, month, trendMonths]);

  // The health-score sparkline is a non-critical fetch: a failure leaves the
  // score card intact. It always shows the last 6 monthly scores, independent
  // of the 6|12 spending-trend selector.
  const fetchHealthScoreTrend = useCallback(async () => {
    const res = await dashboardApi
      .getHealthScoreTrend(year, month, HEALTH_SCORE_TREND_MONTHS)
      .catch(() => null);
    setData((prev) => ({ ...prev, healthScoreTrend: res?.trends ?? null }));
  }, [year, month]);

  useEffect(() => {
    fetchDashboardData();
  }, [fetchDashboardData]);

  useEffect(() => {
    fetchTrendData();
  }, [fetchTrendData]);

  useEffect(() => {
    fetchHealthScoreTrend();
  }, [fetchHealthScoreTrend]);

  return {
    data,
    loading,
    refresh: fetchDashboardData,
    trendMonths,
    setTrendMonths,
  };
}
