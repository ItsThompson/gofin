import { useState, useCallback } from "react";
import { useSearchParams } from "react-router";

export interface ExpenseFilters {
  selectedTypes: Set<string>;
  selectedTags: Set<string>;
  showFilters: boolean;
  dateFrom: string;
  dateTo: string;
  hasActiveFilters: boolean;
  toggleType: (type: string) => void;
  toggleTag: (tagId: string) => void;
  toggleFilters: () => void;
  clearFilters: () => void;
  setDateFrom: (value: string) => void;
  setDateTo: (value: string) => void;
}

/**
 * Encapsulates expense filter state with URL search param sync.
 * Tag filter state is synced to/from the `?tag=` URL parameter.
 */
export function useExpenseFilters(): ExpenseFilters {
  const [searchParams, setSearchParams] = useSearchParams();

  const [showFilters, setShowFilters] = useState(() => searchParams.has("tag"));
  const [selectedTypes, setSelectedTypes] = useState<Set<string>>(new Set());
  const [selectedTags, setSelectedTags] = useState<Set<string>>(() => {
    const tagParam = searchParams.get("tag");
    return tagParam ? new Set([tagParam]) : new Set();
  });
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  const toggleType = useCallback((type: string) => {
    setSelectedTypes((prev) => {
      const next = new Set(prev);
      if (next.has(type)) {
        next.delete(type);
      } else {
        next.add(type);
      }
      return next;
    });
  }, []);

  const toggleTag = useCallback(
    (tagId: string) => {
      setSelectedTags((prev) => {
        const next = new Set(prev);
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
        return next;
      });
    },
    [searchParams, setSearchParams],
  );

  const toggleFilters = useCallback(() => {
    setShowFilters((prev) => !prev);
  }, []);

  const clearFilters = useCallback(() => {
    setSelectedTypes(new Set());
    setSelectedTags(new Set());
    setDateFrom("");
    setDateTo("");
    setSearchParams({}, { replace: true });
  }, [setSearchParams]);

  const hasActiveFilters =
    selectedTypes.size > 0 ||
    selectedTags.size > 0 ||
    dateFrom !== "" ||
    dateTo !== "";

  return {
    selectedTypes,
    selectedTags,
    showFilters,
    dateFrom,
    dateTo,
    hasActiveFilters,
    toggleType,
    toggleTag,
    toggleFilters,
    clearFilters,
    setDateFrom,
    setDateTo,
  };
}
