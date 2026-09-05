import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useApiToast } from "@gofin/api";
import type { BudgetPeriod, Expense, Tag } from "@gofin/core";
import { expenseLogApi } from "../api";
import { resolveTagNames, type ExpenseRow } from "../../../lib/expense-table-columns";
import type { FilterCriteria } from "./useExpenseFilters";

const EXPENSE_LOG_LOAD_ERROR = "Failed to load expense log.";

/** Result of the data fetch, grouped for atomic replacement. */
export interface ExpenseLogFetchResult {
  rawExpenses: Expense[];
  tags: Tag[];
  periods: BudgetPeriod[];
}

export type ExpenseLogDataState =
  | { status: "loading" }
  | { status: "missing"; periods: BudgetPeriod[] }
  | { status: "error"; message: string }
  | {
      status: "active";
      expenses: ExpenseRow[];
      tags: Tag[];
      periods: BudgetPeriod[];
      reportingCurrencyCode: string;
    };

export interface ExpenseLogData {
  state: ExpenseLogDataState;
  selectedYear: number;
  selectedMonth: number;
  setSelectedYear: (year: number) => void;
  setSelectedMonth: (month: number) => void;
  refresh: () => void;
}

/**
 * Encapsulates expense log data fetching, year/month selection, and
 * filtering. Returns a discriminated union so callers cannot hold
 * contradictory flags (loading + active, missing + error) at once.
 */
export function useExpenseLogData(filters: FilterCriteria): ExpenseLogData {
  const now = new Date();
  const [selectedYear, setSelectedYear] = useState(now.getFullYear());
  const [selectedMonth, setSelectedMonth] = useState(now.getMonth() + 1);

  const [fetchResult, setFetchResult] = useState<ExpenseLogFetchResult>({
    rawExpenses: [],
    tags: [],
    periods: [],
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Track whether this is the initial mount to avoid duplicate loading=true
  const isInitialMount = useRef(true);

  const { call: toastCall } = useApiToast<{
    expensesResponse: { data: Expense[] };
    tagsResponse: { tags: Tag[] };
    periodsResponse: { periods: BudgetPeriod[] };
  }>();

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
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
      setFetchResult({
        rawExpenses: result.expensesResponse.data,
        tags: result.tagsResponse.tags,
        periods: result.periodsResponse.periods,
      });
    } else {
      setError(EXPENSE_LOG_LOAD_ERROR);
    }
    setLoading(false);
  }, [selectedYear, selectedMonth, toastCall]);

  // When selection changes, set loading=true immediately before the fetch effect fires
  useEffect(() => {
    if (isInitialMount.current) {
      isInitialMount.current = false;
      return;
    }
    setLoading(true);
  }, [selectedYear, selectedMonth]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const expenses = useMemo(() => {
    const rows = resolveTagNames(fetchResult.rawExpenses, fetchResult.tags);

    return rows.filter((row) => {
      if (filters.selectedTypes.size > 0 && !filters.selectedTypes.has(row.expenseType)) {
        return false;
      }
      if (filters.selectedTags.size > 0 && !filters.selectedTags.has(row.tagId)) {
        return false;
      }
      if (filters.dateFrom && row.expenseDateIso < filters.dateFrom) {
        return false;
      }
      if (filters.dateTo && row.expenseDateIso > filters.dateTo) {
        return false;
      }
      if (filters.selectedTransactionCurrencies.size > 0 && !filters.selectedTransactionCurrencies.has(row.transactionCurrencyEffective)) {
        return false;
      }
      if (filters.selectedReportingCurrencies.size > 0 && !filters.selectedReportingCurrencies.has(row.reportingCurrencyEffective)) {
        return false;
      }
      return true;
    });
  }, [fetchResult.rawExpenses, fetchResult.tags, filters.selectedTypes, filters.selectedTags, filters.dateFrom, filters.dateTo, filters.selectedTransactionCurrencies, filters.selectedReportingCurrencies]);

  const state = useMemo<ExpenseLogDataState>(() => {
    if (loading) return { status: "loading" };
    if (error) return { status: "error", message: error };
    const selected = fetchResult.periods.find(
      (period) => period.year === selectedYear && period.month === selectedMonth,
    );
    if (!selected) return { status: "missing", periods: fetchResult.periods };
    return {
      status: "active",
      expenses,
      tags: fetchResult.tags,
      periods: fetchResult.periods,
      reportingCurrencyCode: selected.reportingCurrencyCode,
    };
  }, [loading, error, fetchResult, selectedYear, selectedMonth, expenses]);

  return {
    state,
    selectedYear,
    selectedMonth,
    setSelectedYear,
    setSelectedMonth,
    refresh: fetchData,
  };
}
