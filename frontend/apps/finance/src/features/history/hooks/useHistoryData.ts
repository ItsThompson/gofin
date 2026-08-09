import { useState, useEffect } from "react";
import { apiClient, useApiToast } from "@gofin/api";
import type { PeriodListResponse, SummaryResponse } from "@gofin/core";
import type { HistoricalPeriodRow, HistoryDataResult } from "../types";

/**
 * Fetches all budget periods and computes totalSpent/surplus for each. A period
 * whose summary fetch fails becomes an unavailable row, so the screen can say
 * the spend is unknown instead of showing a figure nobody measured.
 * Uses useApiToast for error handling on the periods list itself.
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
          allPeriods.map(async (period): Promise<HistoricalPeriodRow> => {
            try {
              const summaryRes = await apiClient<SummaryResponse>(
                `/api/finance/summary?year=${period.year}&month=${period.month}`,
              );
              const totalSpent = summaryRes.summary.totalSpent;
              return {
                period,
                status: "loaded",
                totalSpent,
                surplus: period.budgetAmount - totalSpent,
              };
            } catch {
              return { period, status: "unavailable" };
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
