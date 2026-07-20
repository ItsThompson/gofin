import { useState, useEffect } from "react";
import { apiClient, useApiToast } from "@gofin/api";
import type {
  BudgetPeriod,
  PeriodListResponse,
  SummaryResponse,
} from "@gofin/core";

export interface HistoricalPeriodRow {
  period: BudgetPeriod;
  totalSpent: number;
  surplus: number;
}

export interface HistoryDataResult {
  /** Historical period rows with computed spending data. */
  periods: HistoricalPeriodRow[];
  /** Whether data is currently being fetched. */
  loading: boolean;
}

/**
 * Fetches all budget periods and computes totalSpent/surplus for each.
 * Uses useApiToast for error handling.
 */
export function useHistoryData(): HistoryDataResult {
  const [periods, setPeriods] = useState<HistoricalPeriodRow[]>([]);
  const [loading, setLoading] = useState(true);
  const { call: toastCall } = useApiToast<HistoricalPeriodRow[]>();

  useEffect(() => {
    async function fetchPeriods() {
      const result = await toastCall(async () => {
        const periodsRes =
          await apiClient<PeriodListResponse>("/api/finance/periods");
        const allPeriods = periodsRes.periods;

        const rows = await Promise.all(
          allPeriods.map(async (period) => {
            try {
              const summaryRes = await apiClient<SummaryResponse>(
                `/api/finance/summary?year=${period.year}&month=${period.month}`,
              );
              const totalSpent = summaryRes.summary.totalSpent;
              return {
                period,
                totalSpent,
                surplus: period.budgetAmount - totalSpent,
              };
            } catch {
              return { period, totalSpent: 0, surplus: period.budgetAmount };
            }
          }),
        );

        return rows;
      });

      if (result) {
        setPeriods(result);
      }
      setLoading(false);
    }
    fetchPeriods();
  }, [toastCall]);

  return { periods, loading };
}
