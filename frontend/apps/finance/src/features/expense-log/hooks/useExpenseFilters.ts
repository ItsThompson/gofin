import { useState, useCallback } from "react";
import { useSearchParams } from "react-router";

/** Immutable filter criteria object. */
export interface FilterCriteria {
  selectedTypes: Set<string>;
  selectedTags: Set<string>;
  selectedTransactionCurrencies: Set<string>;
  selectedReportingCurrencies: Set<string>;
  dateFrom: string;
  dateTo: string;
}

/** Empty state constant for reset operations. */
export const EMPTY_CRITERIA: FilterCriteria = {
  selectedTypes: new Set(),
  selectedTags: new Set(),
  selectedTransactionCurrencies: new Set(),
  selectedReportingCurrencies: new Set(),
  dateFrom: "",
  dateTo: "",
};

export interface ExpenseFilters {
  /** Current filter criteria. */
  criteria: FilterCriteria;
  /** Whether the filter panel is visible. */
  showFilters: boolean;
  /** Derived: true if any filter is active. */
  hasActiveFilters: boolean;
  /** Toggle a type in/out of the selected set. */
  toggleType: (type: string) => void;
  /** Toggle a tag in/out of the selected set. Syncs URL param. */
  toggleTag: (tagId: string) => void;
  /** Toggle a transaction currency in/out of the selected set. */
  toggleTransactionCurrency: (currency: string) => void;
  /** Toggle a reporting currency in/out of the selected set. */
  toggleReportingCurrency: (currency: string) => void;
  /** Toggle filter panel visibility. */
  toggleFilters: () => void;
  /** Reset all filters to empty. */
  clearFilters: () => void;
  /** Set the start date filter. */
  setDateFrom: (value: string) => void;
  /** Set the end date filter. */
  setDateTo: (value: string) => void;
}

/**
 * Encapsulates expense filter state with URL search param sync.
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

  const toggleType = useCallback((type: string) => {
    setCriteria((prev) => {
      const next = new Set(prev.selectedTypes);
      if (next.has(type)) {
        next.delete(type);
      } else {
        next.add(type);
      }
      return { ...prev, selectedTypes: next };
    });
  }, []);

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

  const toggleTransactionCurrency = useCallback((currency: string) => {
    setCriteria((prev) => {
      const next = new Set(prev.selectedTransactionCurrencies);
      if (next.has(currency)) {
        next.delete(currency);
      } else {
        next.add(currency);
      }
      return { ...prev, selectedTransactionCurrencies: next };
    });
  }, []);

  const toggleReportingCurrency = useCallback((currency: string) => {
    setCriteria((prev) => {
      const next = new Set(prev.selectedReportingCurrencies);
      if (next.has(currency)) {
        next.delete(currency);
      } else {
        next.add(currency);
      }
      return { ...prev, selectedReportingCurrencies: next };
    });
  }, []);

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
