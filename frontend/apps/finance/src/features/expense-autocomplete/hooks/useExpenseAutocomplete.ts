import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { expenseSuggestionsApi } from "../api";
import type {
  ExpenseAutocompleteActions,
  ExpenseAutocompleteState,
  ExpenseSuggestion,
  ExpenseSuggestionsResponse,
} from "../types";

const INITIAL_PAGE = 1;
const PAGE_SIZE = 50;
const MAX_VISIBLE_SUGGESTIONS = 5;

function dedupeSuggestions(
  currentCandidates: ExpenseSuggestion[],
  newCandidates: ExpenseSuggestion[],
): ExpenseSuggestion[] {
  const seenNames = new Set(currentCandidates.map((candidate) => candidate.name));
  const dedupedCandidates = [...currentCandidates];

  for (const candidate of newCandidates) {
    if (seenNames.has(candidate.name)) {
      continue;
    }

    seenNames.add(candidate.name);
    dedupedCandidates.push(candidate);
  }

  return dedupedCandidates;
}

function getMatchScore(candidateName: string, query: string): number | null {
  const normalizedName = candidateName.toLowerCase();
  const normalizedQuery = query.trim().toLowerCase();

  if (!normalizedQuery) {
    return null;
  }

  if (normalizedName.includes(normalizedQuery)) {
    return normalizedName.indexOf(normalizedQuery);
  }

  let searchFromIndex = 0;
  let score = 0;

  for (const queryCharacter of normalizedQuery) {
    const matchedIndex = normalizedName.indexOf(queryCharacter, searchFromIndex);
    if (matchedIndex === -1) {
      return null;
    }

    score += matchedIndex - searchFromIndex + 1;
    searchFromIndex = matchedIndex + 1;
  }

  return score + normalizedName.length;
}

function getVisibleSuggestions(
  candidates: ExpenseSuggestion[],
  query: string,
): ExpenseSuggestion[] {
  return candidates
    .reduce<Array<{ suggestion: ExpenseSuggestion; score: number; index: number }>>(
      (matches, candidate, index) => {
        const score = getMatchScore(candidate.name, query);
        if (score === null) {
          return matches;
        }

        matches.push({ suggestion: candidate, score, index });
        return matches;
      },
      [],
    )
    .sort((leftMatch, rightMatch) => {
      if (leftMatch.score !== rightMatch.score) {
        return leftMatch.score - rightMatch.score;
      }

      return leftMatch.index - rightMatch.index;
    })
    .slice(0, MAX_VISIBLE_SUGGESTIONS)
    .map((match) => match.suggestion);
}

function getErrorMessage(error: unknown): string | null {
  if (error instanceof DOMException && error.name === "AbortError") {
    return null;
  }

  return "Suggestions are unavailable right now.";
}

export function useExpenseAutocomplete(): {
  state: ExpenseAutocompleteState;
  actions: ExpenseAutocompleteActions;
} {
  const isMountedRef = useRef(false);
  const [query, setQuery] = useState("");
  const [candidates, setCandidates] = useState<ExpenseSuggestion[]>([]);
  const [page, setPage] = useState(0);
  const [isInitialLoading, setIsInitialLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const applyResponse = useCallback((response: ExpenseSuggestionsResponse) => {
    setCandidates((currentCandidates) =>
      dedupeSuggestions(currentCandidates, response.data),
    );
    setPage(response.page);
    setHasMore(response.hasMore);
    setError(null);
  }, []);

  useEffect(() => {
    isMountedRef.current = true;
    const abortController = new AbortController();

    async function loadInitialPage() {
      try {
        const response = await expenseSuggestionsApi.getSuggestions(
          INITIAL_PAGE,
          PAGE_SIZE,
          abortController.signal,
        );

        if (!isMountedRef.current) {
          return;
        }

        applyResponse(response);
      } catch (loadError) {
        if (!isMountedRef.current) {
          return;
        }

        const message = getErrorMessage(loadError);
        if (message) {
          setCandidates([]);
          setError(message);
        }
      } finally {
        if (isMountedRef.current) {
          setIsInitialLoading(false);
        }
      }
    }

    void loadInitialPage();

    return () => {
      isMountedRef.current = false;
      abortController.abort();
    };
  }, [applyResponse]);

  const loadMore = useCallback(async () => {
    if (isLoadingMore || !hasMore) {
      return;
    }

    setIsLoadingMore(true);
    try {
      const response = await expenseSuggestionsApi.getSuggestions(page + 1, PAGE_SIZE);
      if (!isMountedRef.current) {
        return;
      }

      applyResponse(response);
    } catch {
      if (isMountedRef.current) {
        setError("Suggestions are unavailable right now.");
      }
    } finally {
      if (isMountedRef.current) {
        setIsLoadingMore(false);
      }
    }
  }, [applyResponse, hasMore, isLoadingMore, page]);

  const visibleSuggestions = useMemo(
    () => getVisibleSuggestions(candidates, query),
    [candidates, query],
  );

  return {
    state: {
      candidates,
      visibleSuggestions,
      page,
      isInitialLoading,
      isLoadingMore,
      hasMore,
      error,
    },
    actions: {
      setQuery,
      loadMore,
      clearError: () => setError(null),
    },
  };
}
