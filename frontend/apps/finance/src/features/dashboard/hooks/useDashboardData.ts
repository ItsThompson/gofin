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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [summary, setSummary] = useState<PeriodSummary | null>(null);
  const [tagSpending, setTagSpending] = useState<TagSpending[]>([]);
  const [cumulativeData, setCumulativeData] = useState<CumulativeSpendPoint[]>(
    [],
  );
  const [recentExpenses, setRecentExpenses] = useState<Expense[]>([]);
  const [comparison, setComparison] = useState<HistoricalComparison | null>(
    null,
  );
  const [upcomingProRata, setUpcomingProRata] = useState<ProRataSchedule[]>([]);
  const [trendData, setTrendData] = useState<TrendPoint[] | null>(null);
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

    if (summaryRes) setSummary(summaryRes.summary);
    if (tagRes) setTagSpending(tagRes.tagSpending);
    if (cumulativeRes) setCumulativeData(cumulativeRes.points);
    if (expensesRes) setRecentExpenses(expensesRes.data);
    if (comparisonRes) setComparison(comparisonRes.comparison);
    if (upcomingRes) setUpcomingProRata(upcomingRes.schedules);

    setLoading(false);
  }, [year, month, toastCall]);

  const fetchTrendData = useCallback(async () => {
    const trendRes = await dashboardApi
      .getTrend(year, month, trendMonths)
      .catch(() => null);
    setTrendData(trendRes?.trends ?? null);
  }, [year, month, trendMonths]);

  useEffect(() => {
    fetchDashboardData();
  }, [fetchDashboardData]);

  useEffect(() => {
    fetchTrendData();
  }, [fetchTrendData]);

  return {
    data: {
      summary,
      tagSpending,
      cumulativeData,
      recentExpenses,
      comparison,
      upcomingProRata,
      trendData,
    },
    loading,
    error,
    refresh: fetchDashboardData,
    trendMonths,
    setTrendMonths,
  };
}
