import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useApiToast } from "@gofin/api";
import type { BudgetPeriod, Expense, Tag } from "@gofin/core";
import { expenseLogApi } from "../api";
import { resolveTagNames, type ExpenseRow } from "../../../lib/expense-table-columns";
import type { FilterCriteria } from "./useExpenseFilters";

/** Result of the data fetch, grouped for atomic replacement. */
export interface ExpenseLogFetchResult {
  rawExpenses: Expense[];
  tags: Tag[];
  periods: BudgetPeriod[];
}

/** Empty state constant for initial state and resets. */
export const EMPTY_FETCH_RESULT: ExpenseLogFetchResult = {
  rawExpenses: [],
  tags: [],
  periods: [],
};

export interface ExpenseLogData {
  expenses: ExpenseRow[];
  tags: Tag[];
  periods: BudgetPeriod[];
  loading: boolean;
  selectedYear: number;
  selectedMonth: number;
  /** Reporting currency of the currently selected period (falls back to a default). */
  reportingCurrency: string;
  setSelectedYear: (year: number) => void;
  setSelectedMonth: (month: number) => void;
  refresh: () => void;
}

/**
 * Encapsulates expense log data fetching, year/month selection, and
 * filtering. Returns resolved expense rows (with tag names) already
 * filtered by the current filter state.
 *
 * Internal state separates selection (year/month) from fetch results
 * (expenses/tags/periods). When selection changes, `loading` is set to
 * `true` immediately so consumers can dim stale content while the new
 * fetch completes.
 */
export function useExpenseLogData(filters: FilterCriteria): ExpenseLogData {
  const now = new Date();
  const [selectedYear, setSelectedYear] = useState(now.getFullYear());
  const [selectedMonth, setSelectedMonth] = useState(now.getMonth() + 1);

  // Grouped fetch-result state: replaced atomically on success
  const [fetchResult, setFetchResult] = useState<ExpenseLogFetchResult>(EMPTY_FETCH_RESULT);
  const [loading, setLoading] = useState(true);

  // Track whether this is the initial mount to avoid duplicate loading=true
  const isInitialMount = useRef(true);

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
      setFetchResult({
        rawExpenses: result.expensesResponse.data,
        tags: result.tagsResponse.tags,
        periods: result.periodsResponse.periods,
      });
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

  // Derive the reporting currency for the currently selected period.
  const reportingCurrency = useMemo(() => {
    const selected = fetchResult.periods.find(
      (p) => p.year === selectedYear && p.month === selectedMonth,
    );
    return selected?.reportingCurrency ?? "USD";
  }, [fetchResult.periods, selectedYear, selectedMonth]);

  const expenses = useMemo(() => {
    const rows = resolveTagNames(fetchResult.rawExpenses, fetchResult.tags, reportingCurrency);

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
      if (filters.selectedTransactionCurrencies.size > 0 && !filters.selectedTransactionCurrencies.has(row.transactionCurrencyEffective)) {
        return false;
      }
      if (filters.selectedReportingCurrencies.size > 0 && !filters.selectedReportingCurrencies.has(row.reportingCurrencyEffective)) {
        return false;
      }
      return true;
    });
  }, [fetchResult.rawExpenses, fetchResult.tags, filters.selectedTypes, filters.selectedTags, filters.dateFrom, filters.dateTo, filters.selectedTransactionCurrencies, filters.selectedReportingCurrencies, reportingCurrency]);

  return {
    expenses,
    tags: fetchResult.tags,
    periods: fetchResult.periods,
    loading,
    selectedYear,
    selectedMonth,
    reportingCurrency,
    setSelectedYear,
    setSelectedMonth,
    refresh: fetchData,
  };
}
