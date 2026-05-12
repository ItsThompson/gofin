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
} from "../../../types";
import type { PaginatedResponse } from "@gofin/core";
import { dashboardApi } from "../api";

export interface DashboardData {
  summary: PeriodSummary | null;
  tagSpending: TagSpending[];
  cumulativeData: CumulativeSpendPoint[];
  recentExpenses: Expense[];
  comparison: HistoricalComparison | null;
  upcomingProRata: ProRataSchedule[];
  trendData: TrendPoint[] | null;
}

export const EMPTY_DASHBOARD_DATA: DashboardData = {
  summary: null,
  tagSpending: [],
  cumulativeData: [],
  recentExpenses: [],
  comparison: null,
  upcomingProRata: [],
  trendData: null,
};

export interface DashboardDataResult {
  data: DashboardData;
  loading: boolean;
  error: string | null;
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
  const [error, setError] = useState<string | null>(null);
  const [trendMonths, setTrendMonths] = useState<6 | 12>(6);
  const { call: toastCall } = useApiToast();

  const fetchDashboardData = useCallback(async () => {
    setLoading(true);
    setError(null);

    // Fetch each section independently so a single endpoint failure
    // doesn't prevent the rest of the dashboard from rendering.
    const [summaryRes, tagRes, cumulativeRes, expensesRes, comparisonRes, upcomingRes] =
      await Promise.all([
        toastCall(() => dashboardApi.getSummary(year, month)) as Promise<SummaryResponse | undefined>,
        toastCall(() => dashboardApi.getTagSpending(year, month)) as Promise<TagSpendingResponse | undefined>,
        toastCall(() => dashboardApi.getCumulative(year, month)) as Promise<CumulativeSpendResponse | undefined>,
        toastCall(() => dashboardApi.getRecentExpenses(year, month, 5)) as Promise<PaginatedResponse<Expense> | undefined>,
        dashboardApi.getComparison(year, month).catch(() => null),
        dashboardApi
          .getUpcomingProRata()
          .catch(() => ({ schedules: [] as ProRataSchedule[] })),
      ]);

    if (summaryRes) setData(prev => ({ ...prev, summary: summaryRes.summary }));
    if (tagRes) setData(prev => ({ ...prev, tagSpending: tagRes.tagSpending }));
    if (cumulativeRes) setData(prev => ({ ...prev, cumulativeData: cumulativeRes.points }));
    if (expensesRes) setData(prev => ({ ...prev, recentExpenses: expensesRes.data }));
    if (comparisonRes) setData(prev => ({ ...prev, comparison: comparisonRes.comparison }));
    if (upcomingRes) setData(prev => ({ ...prev, upcomingProRata: upcomingRes.schedules }));

    setLoading(false);
  }, [year, month, toastCall]);

  const fetchTrendData = useCallback(async () => {
    const trendRes = await dashboardApi
      .getTrend(year, month, trendMonths)
      .catch(() => null);
    setData(prev => ({ ...prev, trendData: trendRes?.trends ?? null }));
  }, [year, month, trendMonths]);

  useEffect(() => {
    fetchDashboardData();
  }, [fetchDashboardData]);

  useEffect(() => {
    fetchTrendData();
  }, [fetchTrendData]);

  return {
    data,
    loading,
    error,
    refresh: fetchDashboardData,
    trendMonths,
    setTrendMonths,
  };
}
