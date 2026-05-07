import { useState, useEffect, useCallback, useMemo } from "react";
import { useApiToast } from "@gofin/api";
import type { BudgetPeriod, Expense, Tag } from "../../../types";
import { expenseLogApi } from "../api";
import { resolveTagNames, type ExpenseRow } from "../../../lib/expense-table-columns";
import type { ExpenseFilters } from "./useExpenseFilters";

export interface ExpenseLogData {
  expenses: ExpenseRow[];
  tags: Tag[];
  periods: BudgetPeriod[];
  loading: boolean;
  selectedYear: number;
  selectedMonth: number;
  setSelectedYear: (year: number) => void;
  setSelectedMonth: (month: number) => void;
  refresh: () => void;
}

/**
 * Encapsulates expense log data fetching, year/month selection, and
 * filtering. Returns resolved expense rows (with tag names) already
 * filtered by the current filter state.
 */
export function useExpenseLogData(filters: ExpenseFilters): ExpenseLogData {
  const now = new Date();
  const [selectedYear, setSelectedYear] = useState(now.getFullYear());
  const [selectedMonth, setSelectedMonth] = useState(now.getMonth() + 1);

  const [rawExpenses, setRawExpenses] = useState<Expense[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [periods, setPeriods] = useState<BudgetPeriod[]>([]);
  const [loading, setLoading] = useState(true);
  const { call: toastCall } = useApiToast<{
    expensesResponse: { data: Expense[] };
    tagsResponse: { tags: Tag[] };
    periodsResponse: { periods: BudgetPeriod[] };
  }>();

  const fetchData = useCallback(async () => {
    setLoading(true);
    const result = await toastCall(async () => {
      const [expensesResponse, tagsResponse, periodsResponse] =
        await Promise.all([
          expenseLogApi.getExpenses(selectedYear, selectedMonth),
          expenseLogApi.getTags(),
          expenseLogApi.getPeriods(),
        ]);
      return { expensesResponse, tagsResponse, periodsResponse };
    });

    if (result) {
      setRawExpenses(result.expensesResponse.data);
      setTags(result.tagsResponse.tags);
      setPeriods(result.periodsResponse.periods);
    }
    setLoading(false);
  }, [selectedYear, selectedMonth, toastCall]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const expenses = useMemo(() => {
    const rows = resolveTagNames(rawExpenses, tags);

    return rows.filter((row) => {
      if (filters.selectedTypes.size > 0 && !filters.selectedTypes.has(row.expenseType)) {
        return false;
      }
      if (filters.selectedTags.size > 0 && !filters.selectedTags.has(row.tagId)) {
        return false;
      }
      if (filters.dateFrom && row.expenseDate < filters.dateFrom) {
        return false;
      }
      if (filters.dateTo && row.expenseDate > filters.dateTo) {
        return false;
      }
      return true;
    });
  }, [rawExpenses, tags, filters.selectedTypes, filters.selectedTags, filters.dateFrom, filters.dateTo]);

  return {
    expenses,
    tags,
    periods,
    loading,
    selectedYear,
    selectedMonth,
    setSelectedYear,
    setSelectedMonth,
    refresh: fetchData,
  };
}
