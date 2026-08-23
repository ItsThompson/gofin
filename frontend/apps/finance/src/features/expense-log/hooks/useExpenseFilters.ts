import { useState, useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";

export interface FilterCriteria {
  selectedTypes: Set<string>;
  selectedTags: Set<string>;
  selectedTransactionCurrencies: Set<string>;
  selectedReportingCurrencies: Set<string>;
  dateFrom: string;
  dateTo: string;
}

export const EMPTY_CRITERIA: FilterCriteria = {
  selectedTypes: new Set(),
  selectedTags: new Set(),
  selectedTransactionCurrencies: new Set(),
  selectedReportingCurrencies: new Set(),
  dateFrom: "",
  dateTo: "",
};

export interface ExpenseFilters {
  criteria: FilterCriteria;
  showFilters: boolean;
  /** Derived from the current criteria. */
  hasActiveFilters: boolean;
  toggleType: (type: string) => void;
  /** Syncs the ?tag= URL param. */
  toggleTag: (tagId: string) => void;
  toggleTransactionCurrency: (currency: string) => void;
  toggleReportingCurrency: (currency: string) => void;
  toggleFilters: () => void;
  clearFilters: () => void;
  setDateFrom: (value: string) => void;
  setDateTo: (value: string) => void;
}

/**
 * Tag filter state is synced to/from the `?tag=` URL parameter.
 */
export function useExpenseFilters(): ExpenseFilters {
  const [searchParams, setSearchParams] = useSearchParams();

  const [showFilters, setShowFilters] = useState(() => searchParams.has("tag"));
  const [criteria, setCriteria] = useState<FilterCriteria>(() => {
    const tagParam = searchParams.get("tag");
    return {
      selectedTypes: new Set(),
      selectedTags: tagParam ? new Set([tagParam]) : new Set(),
      selectedTransactionCurrencies: new Set(),
      selectedReportingCurrencies: new Set(),
      dateFrom: "",
      dateTo: "",
    };
  });

  const toggleSetField = useCallback(
    (
      field:
        | "selectedTypes"
        | "selectedTransactionCurrencies"
        | "selectedReportingCurrencies",
    ) =>
      (value: string) => {
        setCriteria((prev) => {
          const next = new Set(prev[field]);
          if (next.has(value)) {
            next.delete(value);
          } else {
            next.add(value);
          }
          return { ...prev, [field]: next };
        });
      },
    [],
  );

  const toggleType = useMemo(() => toggleSetField("selectedTypes"), [toggleSetField]);
  const toggleTransactionCurrency = useMemo(
    () => toggleSetField("selectedTransactionCurrencies"),
    [toggleSetField],
  );
  const toggleReportingCurrency = useMemo(
    () => toggleSetField("selectedReportingCurrencies"),
    [toggleSetField],
  );

  const toggleTag = useCallback(
    (tagId: string) => {
      setCriteria((prev) => {
        const next = new Set(prev.selectedTags);
        if (next.has(tagId)) {
          next.delete(tagId);
        } else {
          next.add(tagId);
        }
        if (next.size === 1) {
          setSearchParams({ tag: [...next][0] }, { replace: true });
        } else {
          const updatedParams = new URLSearchParams(searchParams);
          updatedParams.delete("tag");
          setSearchParams(updatedParams, { replace: true });
        }
        return { ...prev, selectedTags: next };
      });
    },
    [searchParams, setSearchParams],
  );

  const toggleFilters = useCallback(() => {
    setShowFilters((prev) => !prev);
  }, []);

  const clearFilters = useCallback(() => {
    setCriteria(EMPTY_CRITERIA);
    setSearchParams({}, { replace: true });
  }, [setSearchParams]);

  const setDateFrom = useCallback((value: string) => {
    setCriteria((prev) => ({ ...prev, dateFrom: value }));
  }, []);

  const setDateTo = useCallback((value: string) => {
    setCriteria((prev) => ({ ...prev, dateTo: value }));
  }, []);

  const hasActiveFilters =
    criteria.selectedTypes.size > 0 ||
    criteria.selectedTags.size > 0 ||
    criteria.selectedTransactionCurrencies.size > 0 ||
    criteria.selectedReportingCurrencies.size > 0 ||
    criteria.dateFrom !== "" ||
    criteria.dateTo !== "";

  return {
    criteria,
    showFilters,
    hasActiveFilters,
    toggleType,
    toggleTag,
    toggleTransactionCurrency,
    toggleReportingCurrency,
    toggleFilters,
    clearFilters,
    setDateFrom,
    setDateTo,
  };
}
